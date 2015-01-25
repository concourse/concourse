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
	plan atc.InputPlan
	info exec.VersionInfo
}

//go:generate counterfeiter . BuildDelegate

type BuildDelegate interface {
	InputDelegate(lager.Logger, atc.InputPlan) exec.GetDelegate
	ExecutionDelegate(lager.Logger) exec.ExecuteDelegate
	OutputDelegate(lager.Logger, atc.OutputPlan) exec.PutDelegate

	Start(lager.Logger)
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

func (delegate *delegate) InputDelegate(logger lager.Logger, plan atc.InputPlan) exec.GetDelegate {
	return &inputDelegate{
		logger:   logger,
		plan:     plan,
		delegate: delegate,
	}
}

func (delegate *delegate) OutputDelegate(logger lager.Logger, plan atc.OutputPlan) exec.PutDelegate {
	return &outputDelegate{
		logger:   logger,
		plan:     plan,
		delegate: delegate,
	}
}

func (delegate *delegate) ExecutionDelegate(logger lager.Logger) exec.ExecuteDelegate {
	return &executionDelegate{
		logger:   logger,
		delegate: delegate,
	}
}

func (delegate *delegate) Start(logger lager.Logger) {
	// TODO?: make this a callback hooked in to the steps when a certain one starts

	startedAt := time.Now()

	logger.Info("start", lager.Data{"started-at": startedAt})

	err := delegate.db.SaveBuildStartTime(delegate.buildID, startedAt)
	if err != nil {
		logger.Error("failed-to-save-start-time", err)
	}

	err = delegate.db.SaveBuildStatus(delegate.buildID, db.StatusStarted)
	if err != nil {
		logger.Error("failed-to-save-status", err)
	}

	err = delegate.db.SaveBuildEvent(delegate.buildID, event.Status{
		Status: atc.StatusStarted,
		Time:   startedAt.Unix(),
	})
	if err != nil {
		logger.Error("failed-to-save-status-event", err)
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

func (delegate *delegate) saveInitialize(logger lager.Logger, buildConfig atc.BuildConfig) {
	err := delegate.db.SaveBuildEvent(delegate.buildID, event.Initialize{
		BuildConfig: buildConfig,
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-event", err)
	}
}

func (delegate *delegate) saveStart(logger lager.Logger) {
	err := delegate.db.SaveBuildEvent(delegate.buildID, event.Start{
		Time: time.Now().Unix(),
	})
	if err != nil {
		logger.Error("failed-to-save-start-event", err)
	}
}

func (delegate *delegate) saveFinish(logger lager.Logger, status exec.ExitStatus) {
	err := delegate.db.SaveBuildEvent(delegate.buildID, event.Finish{
		ExitStatus: int(status),
		Time:       time.Now().Unix(),
	})
	if err != nil {
		logger.Error("failed-to-save-finish-event", err)
	}
}

func (delegate *delegate) saveStatus(logger lager.Logger, status atc.BuildStatus) {
	time := time.Now()

	err := delegate.db.SaveBuildEndTime(delegate.buildID, time)
	if err != nil {
		logger.Error("failed-to-save-end-time", err)
	}

	err = delegate.db.SaveBuildStatus(delegate.buildID, db.Status(status))
	if err != nil {
		logger.Error("failed-to-save-status", err)
	}

	err = delegate.db.SaveBuildEvent(delegate.buildID, event.Status{
		Status: status,
		Time:   time.Unix(),
	})
	if err != nil {
		logger.Error("failed-to-save-status-event", err)
	}

	err = delegate.db.CompleteBuild(delegate.buildID)
	if err != nil {
		logger.Error("failed-to-complete-build", err)
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

func (delegate *delegate) saveInput(logger lager.Logger, plan atc.InputPlan, info exec.VersionInfo) {
	ev := event.Input{
		Plan:            plan,
		FetchedVersion:  info.Version,
		FetchedMetadata: info.Metadata,
	}

	err := delegate.db.SaveBuildEvent(delegate.buildID, ev)
	if err != nil {
		logger.Error("failed-to-save-input-event", err)
	}

	_, err = delegate.db.SaveBuildInput(delegate.buildID, db.BuildInput{
		Name:              plan.Name,
		VersionedResource: vrFromInput(ev),
	})
	if err != nil {
		logger.Error("failed-to-save-input", err)
	}
}

func (delegate *delegate) saveOutput(logger lager.Logger, plan atc.OutputPlan, info exec.VersionInfo) {
	ev := event.Output{
		Plan:            plan,
		CreatedVersion:  info.Version,
		CreatedMetadata: info.Metadata,
	}

	err := delegate.db.SaveBuildEvent(delegate.buildID, ev)
	if err != nil {
		logger.Error("failed-to-save-output-event", err)
	}

	_, err = delegate.db.SaveBuildOutput(delegate.buildID, vrFromOutput(ev))
	if err != nil {
		logger.Error("failed-to-save-output", err)
	}
}

func (delegate *delegate) saveImplicitOutput(logger lager.Logger, plan atc.InputPlan, info exec.VersionInfo) {
	metadata := make([]db.MetadataField, len(info.Metadata))
	for i, md := range info.Metadata {
		metadata[i] = db.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	_, err := delegate.db.SaveBuildOutput(delegate.buildID, db.VersionedResource{
		Resource: plan.Resource,
		Type:     plan.Type,
		Source:   db.Source(plan.Source),
		Version:  db.Version(info.Version),
		Metadata: metadata,
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

	plan atc.InputPlan

	delegate *delegate
}

func (input *inputDelegate) Completed(info exec.VersionInfo) {
	input.delegate.saveInput(input.logger, input.plan, info)
	input.delegate.registerImplicitOutput(input.plan.Resource, implicitOutput{input.plan, info})
	input.logger.Info("finished", lager.Data{"version-info": info})
}

func (input *inputDelegate) Failed(err error) {
	input.delegate.saveErr(input.logger, err, event.Origin{
		Type: event.OriginTypeInput,
		Name: input.plan.Name,
	})

	input.logger.Error("errored", err)
}

func (input *inputDelegate) Stdout() io.Writer {
	return input.delegate.eventWriter(event.Origin{
		Type: event.OriginTypeInput,
		Name: input.plan.Name,
	})
}

func (input *inputDelegate) Stderr() io.Writer {
	return input.delegate.eventWriter(event.Origin{
		Type: event.OriginTypeInput,
		Name: input.plan.Name,
	})
}

type outputDelegate struct {
	logger lager.Logger

	plan atc.OutputPlan

	delegate *delegate
}

func (output *outputDelegate) Completed(info exec.VersionInfo) {
	output.delegate.unregisterImplicitOutput(output.plan.Name)
	output.delegate.saveOutput(output.logger, output.plan, info)
	output.logger.Info("finished", lager.Data{"version-info": info})
}

func (output *outputDelegate) Failed(err error) {
	output.delegate.saveErr(output.logger, err, event.Origin{
		Type: event.OriginTypeOutput,
		Name: output.plan.Name,
	})

	output.logger.Error("errored", err)
}

func (output *outputDelegate) Stdout() io.Writer {
	return output.delegate.eventWriter(event.Origin{
		Type: event.OriginTypeOutput,
		Name: output.plan.Name,
	})
}

func (output *outputDelegate) Stderr() io.Writer {
	return output.delegate.eventWriter(event.Origin{
		Type: event.OriginTypeOutput,
		Name: output.plan.Name,
	})
}

type executionDelegate struct {
	logger lager.Logger

	delegate *delegate
}

func (execution *executionDelegate) Initializing(config atc.BuildConfig) {
	execution.delegate.saveInitialize(execution.logger, config)
}

func (execution *executionDelegate) Started() {
	execution.delegate.saveStart(execution.logger)

	execution.logger.Info("started")
}

func (execution *executionDelegate) Finished(status exec.ExitStatus) {
	execution.delegate.saveFinish(execution.logger, status)

	if status != 0 {
		execution.delegate.successful = false
	}

	execution.logger.Info("finished", lager.Data{
		"status":    status,
		"succeeded": status == 0,
	})
}

func (execution *executionDelegate) Failed(err error) {
	execution.delegate.saveErr(execution.logger, err, event.Origin{})
	execution.logger.Error("errored", err)
}

func (execution *executionDelegate) Stdout() io.Writer {
	return execution.delegate.eventWriter(event.Origin{
		Type: event.OriginTypeRun,
		Name: "stdout",
	})
}

func (execution *executionDelegate) Stderr() io.Writer {
	return execution.delegate.eventWriter(event.Origin{
		Type: event.OriginTypeRun,
		Name: "stderr",
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

func vrFromInput(input event.Input) db.VersionedResource {
	metadata := make([]db.MetadataField, len(input.FetchedMetadata))
	for i, md := range input.FetchedMetadata {
		metadata[i] = db.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	return db.VersionedResource{
		Resource: input.Plan.Resource,
		Type:     input.Plan.Type,
		Source:   db.Source(input.Plan.Source),
		Version:  db.Version(input.FetchedVersion),
		Metadata: metadata,
	}
}

func vrFromOutput(output event.Output) db.VersionedResource {
	metadata := make([]db.MetadataField, len(output.CreatedMetadata))
	for i, md := range output.CreatedMetadata {
		metadata[i] = db.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	return db.VersionedResource{
		Resource: output.Plan.Name,
		Type:     output.Plan.Type,
		Source:   db.Source(output.Plan.Source),
		Version:  db.Version(output.CreatedVersion),
		Metadata: metadata,
	}
}
