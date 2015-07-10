package engine

import (
	"io"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
	"github.com/pivotal-golang/lager"
)

type implicitOutput struct {
	plan atc.GetPlan
	info exec.VersionInfo
}

//go:generate counterfeiter . BuildDelegate

type BuildDelegate interface {
	InputDelegate(lager.Logger, atc.GetPlan, event.OriginLocation, string) exec.GetDelegate
	ExecutionDelegate(lager.Logger, atc.TaskPlan, event.OriginLocation, string) exec.TaskDelegate
	OutputDelegate(lager.Logger, atc.PutPlan, event.OriginLocation, string) exec.PutDelegate

	Finish(lager.Logger, error)
	Aborted(lager.Logger)
}

//go:generate counterfeiter . BuildDelegateFactory

type BuildDelegateFactory interface {
	Delegate(buildID int) BuildDelegate
}

type buildDelegateFactory struct {
	db EngineDB
}

func NewBuildDelegateFactory(db EngineDB) BuildDelegateFactory {
	return buildDelegateFactory{db}
}

func (factory buildDelegateFactory) Delegate(buildID int) BuildDelegate {
	return newBuildDelegate(factory.db, buildID)
}

type delegate struct {
	db EngineDB

	buildID int

	successful bool
	aborted    bool

	implicitOutputs map[string]implicitOutput

	lock sync.Mutex
}

func newBuildDelegate(db EngineDB, buildID int) BuildDelegate {
	return &delegate{
		db: db,

		buildID: buildID,

		successful: true,
		aborted:    false,

		implicitOutputs: make(map[string]implicitOutput),
	}
}

func (delegate *delegate) InputDelegate(logger lager.Logger, plan atc.GetPlan, location event.OriginLocation, hook string) exec.GetDelegate {
	return &inputDelegate{
		logger:   logger,
		plan:     plan,
		location: location,
		delegate: delegate,
		hook:     hook,
	}
}

func (delegate *delegate) OutputDelegate(logger lager.Logger, plan atc.PutPlan, location event.OriginLocation, hook string) exec.PutDelegate {
	return &outputDelegate{
		logger:   logger,
		plan:     plan,
		location: location,
		delegate: delegate,
		hook:     hook,
	}
}

func (delegate *delegate) ExecutionDelegate(logger lager.Logger, plan atc.TaskPlan, location event.OriginLocation, hook string) exec.TaskDelegate {
	return &executionDelegate{
		logger:   logger,
		plan:     plan,
		location: location,
		delegate: delegate,
		hook:     hook,
	}
}

func (delegate *delegate) Finish(logger lager.Logger, err error) {
	if delegate.aborted {
		delegate.saveStatus(logger, atc.StatusAborted)

		logger.Info("aborted")
	} else if err != nil {
		delegate.saveStatus(logger, atc.StatusErrored)

		logger.Error("errored", err)
	} else if delegate.successful {
		delegate.saveStatus(logger, atc.StatusSucceeded)

		implicits := logger.Session("implicit-outputs")

		for _, o := range delegate.implicitOutputs {
			delegate.saveImplicitOutput(implicits.Session(o.plan.Name), o.plan, o.info)
		}

		logger.Info("succeeded")
	} else {
		delegate.saveStatus(logger, atc.StatusFailed)

		logger.Info("failed")
	}
}

func (delegate *delegate) Aborted(logger lager.Logger) {
	delegate.aborted = true

	logger.Info("aborted")
}

func (delegate *delegate) registerImplicitOutput(resource string, output implicitOutput) {
	delegate.lock.Lock()
	delegate.implicitOutputs[resource] = output
	delegate.lock.Unlock()
}

func (delegate *delegate) unregisterImplicitOutput(resource string) {
	delegate.lock.Lock()
	delete(delegate.implicitOutputs, resource)
	delegate.lock.Unlock()
}

func (delegate *delegate) saveInitialize(logger lager.Logger, taskConfig atc.TaskConfig, origin event.Origin) {
	err := delegate.db.SaveBuildEvent(delegate.buildID, event.InitializeTask{
		TaskConfig: taskConfig,
		Origin:     origin,
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-event", err)
	}
}

func (delegate *delegate) saveStart(logger lager.Logger, origin event.Origin) {
	err := delegate.db.SaveBuildEvent(delegate.buildID, event.StartTask{
		Time:   time.Now().Unix(),
		Origin: origin,
	})
	if err != nil {
		logger.Error("failed-to-save-start-event", err)
	}
}

func (delegate *delegate) saveFinish(logger lager.Logger, status exec.ExitStatus, origin event.Origin) {
	err := delegate.db.SaveBuildEvent(delegate.buildID, event.FinishTask{
		ExitStatus: int(status),
		Time:       time.Now().Unix(),
		Origin:     origin,
	})
	if err != nil {
		logger.Error("failed-to-save-finish-event", err)
	}
}

func (delegate *delegate) saveStatus(logger lager.Logger, status atc.BuildStatus) {
	err := delegate.db.FinishBuild(delegate.buildID, db.Status(status))
	if err != nil {
		logger.Error("failed-to-finish-build", err)
	}
}

func (delegate *delegate) saveErr(logger lager.Logger, errVal error, origin event.Origin) {
	err := delegate.db.SaveBuildEvent(delegate.buildID, event.Error{
		Message: errVal.Error(),
		Origin:  origin,
	})
	if err != nil {
		logger.Error("failed-to-save-error-event", err)
	}
}

func (delegate *delegate) saveInput(logger lager.Logger, status exec.ExitStatus, plan atc.GetPlan, info exec.VersionInfo, origin event.Origin) {
	ev := event.FinishGet{
		Origin: origin,
		Plan: event.GetPlan{
			Name:     plan.Name,
			Resource: plan.Resource,
			Type:     plan.Type,
			Source:   plan.Source,
			Params:   plan.Params,
			Version:  plan.Version,
		},
		ExitStatus:      int(status),
		FetchedVersion:  info.Version,
		FetchedMetadata: info.Metadata,
	}

	err := delegate.db.SaveBuildEvent(delegate.buildID, ev)
	if err != nil {
		logger.Error("failed-to-save-input-event", err)
	}

	_, err = delegate.db.SaveBuildInput(delegate.buildID, db.BuildInput{
		Name:              plan.Name,
		VersionedResource: vrFromInput(plan.Pipeline, ev),
	})
	if err != nil {
		logger.Error("failed-to-save-input", err)
	}
}

func (delegate *delegate) saveOutput(logger lager.Logger, status exec.ExitStatus, plan atc.PutPlan, info exec.VersionInfo, origin event.Origin) {
	ev := event.FinishPut{
		Origin: origin,
		Plan: event.PutPlan{
			Name:     plan.Name,
			Resource: plan.Resource,
			Type:     plan.Type,
			Source:   plan.Source,
			Params:   plan.Params,
		},
		ExitStatus:      int(status),
		CreatedVersion:  info.Version,
		CreatedMetadata: info.Metadata,
	}

	err := delegate.db.SaveBuildEvent(delegate.buildID, ev)
	if err != nil {
		logger.Error("failed-to-save-output-event", err)
	}

	_, err = delegate.db.SaveBuildOutput(delegate.buildID, vrFromOutput(plan.Pipeline, ev))
	if err != nil {
		logger.Error("failed-to-save-output", err)
	}
}

func (delegate *delegate) saveImplicitOutput(logger lager.Logger, plan atc.GetPlan, info exec.VersionInfo) {
	metadata := make([]db.MetadataField, len(info.Metadata))
	for i, md := range info.Metadata {
		metadata[i] = db.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	_, err := delegate.db.SaveBuildOutput(delegate.buildID, db.VersionedResource{
		PipelineName: plan.Pipeline,
		Resource:     plan.Resource,
		Type:         plan.Type,
		Source:       db.Source(plan.Source),
		Version:      db.Version(info.Version),
		Metadata:     metadata,
	})
	if err != nil {
		logger.Error("failed-to-save", err)
		return
	}

	logger.Info("saved", lager.Data{"resource": plan.Resource})
}

func (delegate *delegate) eventWriter(origin event.Origin) io.Writer {
	return &dbEventWriter{
		db:      delegate.db,
		buildID: delegate.buildID,
		origin:  origin,
	}
}

type inputDelegate struct {
	logger lager.Logger

	plan     atc.GetPlan
	location event.OriginLocation
	hook     string
	delegate *delegate
}

func (input *inputDelegate) Completed(status exec.ExitStatus, info exec.VersionInfo) {
	if status != 0 {
		input.delegate.successful = false
	}

	input.delegate.saveInput(input.logger, status, input.plan, info, event.Origin{
		Type:     event.OriginTypeGet,
		Name:     input.plan.Name,
		Location: input.location,
		Hook:     input.hook,
	})
	input.delegate.registerImplicitOutput(input.plan.Resource, implicitOutput{input.plan, info})
	input.logger.Info("finished", lager.Data{"version-info": info})
}

func (input *inputDelegate) Failed(err error) {
	input.delegate.saveErr(input.logger, err, event.Origin{
		Type:     event.OriginTypeGet,
		Name:     input.plan.Name,
		Location: input.location,
		Hook:     input.hook,
	})

	input.logger.Error("errored", err)
}

func (input *inputDelegate) Stdout() io.Writer {
	return input.delegate.eventWriter(event.Origin{
		Type:     event.OriginTypeGet,
		Name:     input.plan.Name,
		Source:   event.OriginSourceStdout,
		Location: input.location,
		Hook:     input.hook,
	})
}

func (input *inputDelegate) Stderr() io.Writer {
	return input.delegate.eventWriter(event.Origin{
		Type:     event.OriginTypeGet,
		Name:     input.plan.Name,
		Source:   event.OriginSourceStderr,
		Location: input.location,
		Hook:     input.hook,
	})
}

type outputDelegate struct {
	logger lager.Logger

	plan     atc.PutPlan
	location event.OriginLocation

	delegate *delegate
	hook     string
}

func (output *outputDelegate) Completed(status exec.ExitStatus, info exec.VersionInfo) {
	if status != 0 {
		output.delegate.successful = false
	}

	output.delegate.unregisterImplicitOutput(output.plan.Resource)
	output.delegate.saveOutput(output.logger, status, output.plan, info, event.Origin{
		Type:     event.OriginTypePut,
		Name:     output.plan.Name,
		Location: output.location,
		Hook:     output.hook,
	})
	output.logger.Info("finished", lager.Data{"version-info": info})
}

func (output *outputDelegate) Failed(err error) {
	output.delegate.saveErr(output.logger, err, event.Origin{
		Type:     event.OriginTypePut,
		Name:     output.plan.Name,
		Location: output.location,
		Hook:     output.hook,
	})

	output.logger.Error("errored", err)
}

func (output *outputDelegate) Stdout() io.Writer {
	return output.delegate.eventWriter(event.Origin{
		Type:     event.OriginTypePut,
		Name:     output.plan.Name,
		Source:   event.OriginSourceStdout,
		Location: output.location,
		Hook:     output.hook,
	})
}

func (output *outputDelegate) Stderr() io.Writer {
	return output.delegate.eventWriter(event.Origin{
		Type:     event.OriginTypePut,
		Name:     output.plan.Name,
		Source:   event.OriginSourceStderr,
		Location: output.location,
		Hook:     output.hook,
	})
}

type executionDelegate struct {
	logger lager.Logger

	plan     atc.TaskPlan
	location event.OriginLocation

	delegate *delegate

	hook string
}

func (execution *executionDelegate) Initializing(config atc.TaskConfig) {
	execution.delegate.saveInitialize(execution.logger, config, event.Origin{
		Type:     event.OriginTypeTask,
		Name:     execution.plan.Name,
		Location: execution.location,
		Hook:     execution.hook,
	})
}

func (execution *executionDelegate) Started() {
	execution.delegate.saveStart(execution.logger, event.Origin{
		Type:     event.OriginTypeTask,
		Name:     execution.plan.Name,
		Location: execution.location,
		Hook:     execution.hook,
	})

	execution.logger.Info("started")
}

func (execution *executionDelegate) Finished(status exec.ExitStatus) {
	execution.delegate.saveFinish(execution.logger, status, event.Origin{
		Type:     event.OriginTypeTask,
		Name:     execution.plan.Name,
		Location: execution.location,
		Hook:     execution.hook,
	})

	execution.Result(status)
}

func (execution *executionDelegate) Result(status exec.ExitStatus) {
	if status != 0 {
		execution.delegate.successful = false
	}

	execution.logger.Info("result", lager.Data{
		"status":    status,
		"succeeded": status == 0,
	})
}

func (execution *executionDelegate) Failed(err error) {
	execution.delegate.saveErr(execution.logger, err, event.Origin{
		Type:     event.OriginTypeTask,
		Name:     execution.plan.Name,
		Location: execution.location,
		Hook:     execution.hook,
	})
	execution.logger.Error("errored", err)
}

func (execution *executionDelegate) Stdout() io.Writer {
	return execution.delegate.eventWriter(event.Origin{
		Type:     event.OriginTypeTask,
		Name:     execution.plan.Name,
		Source:   event.OriginSourceStdout,
		Location: execution.location,
		Hook:     execution.hook,
	})
}

func (execution *executionDelegate) Stderr() io.Writer {
	return execution.delegate.eventWriter(event.Origin{
		Type:     event.OriginTypeTask,
		Name:     execution.plan.Name,
		Source:   event.OriginSourceStderr,
		Location: execution.location,
		Hook:     execution.hook,
	})
}

type dbEventWriter struct {
	buildID int
	db      EngineDB

	origin event.Origin

	dangling []byte
}

func (writer *dbEventWriter) Write(data []byte) (int, error) {
	text := append(writer.dangling, data...)

	checkEncoding, _ := utf8.DecodeLastRune(text)
	if checkEncoding == utf8.RuneError {
		writer.dangling = text
		return len(data), nil
	}

	writer.dangling = nil

	writer.db.SaveBuildEvent(writer.buildID, event.Log{
		Payload: string(text),
		Origin:  writer.origin,
	})

	return len(data), nil
}

func vrFromInput(pipelineName string, got event.FinishGet) db.VersionedResource {
	metadata := make([]db.MetadataField, len(got.FetchedMetadata))
	for i, md := range got.FetchedMetadata {
		metadata[i] = db.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	return db.VersionedResource{
		Resource:     got.Plan.Resource,
		PipelineName: pipelineName,
		Type:         got.Plan.Type,
		Source:       db.Source(got.Plan.Source),
		Version:      db.Version(got.FetchedVersion),
		Metadata:     metadata,
	}
}

func vrFromOutput(pipelineName string, putted event.FinishPut) db.VersionedResource {
	metadata := make([]db.MetadataField, len(putted.CreatedMetadata))
	for i, md := range putted.CreatedMetadata {
		metadata[i] = db.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	return db.VersionedResource{
		Resource:     putted.Plan.Resource,
		PipelineName: pipelineName,
		Type:         putted.Plan.Type,
		Source:       db.Source(putted.Plan.Source),
		Version:      db.Version(putted.CreatedVersion),
		Metadata:     metadata,
	}
}
