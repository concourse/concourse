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
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager"
)

type implicitOutput struct {
	plan atc.GetPlan
	info exec.VersionInfo
}

//go:generate counterfeiter . BuildDelegate

type BuildDelegate interface {
	InputDelegate(lager.Logger, atc.GetPlan, event.OriginID) exec.GetDelegate
	ExecutionDelegate(lager.Logger, atc.TaskPlan, event.OriginID) exec.TaskDelegate
	OutputDelegate(lager.Logger, atc.PutPlan, event.OriginID) exec.PutDelegate

	Finish(lager.Logger, error, exec.Success, bool)
}

//go:generate counterfeiter . BuildDelegateFactory

type BuildDelegateFactory interface {
	Delegate(buildID int, pipelineID int) BuildDelegate
}

type buildDelegateFactory struct {
	db EngineDB
}

func NewBuildDelegateFactory(db EngineDB) BuildDelegateFactory {
	return buildDelegateFactory{db}
}

func (factory buildDelegateFactory) Delegate(buildID int, pipelineID int) BuildDelegate {
	return newBuildDelegate(factory.db, buildID, pipelineID)
}

type delegate struct {
	db EngineDB

	buildID    int
	pipelineID int

	implicitOutputs map[string]implicitOutput

	lock sync.Mutex
}

func newBuildDelegate(db EngineDB, buildID int, pipelineID int) BuildDelegate {
	return &delegate{
		db: db,

		buildID:    buildID,
		pipelineID: pipelineID,

		implicitOutputs: make(map[string]implicitOutput),
	}
}

func (delegate *delegate) InputDelegate(logger lager.Logger, plan atc.GetPlan, id event.OriginID) exec.GetDelegate {
	return &inputDelegate{
		logger: logger,

		id:       id,
		plan:     plan,
		delegate: delegate,
	}
}

func (delegate *delegate) OutputDelegate(logger lager.Logger, plan atc.PutPlan, id event.OriginID) exec.PutDelegate {
	return &outputDelegate{
		logger: logger,

		id:       id,
		plan:     plan,
		delegate: delegate,
	}
}

func (delegate *delegate) ExecutionDelegate(logger lager.Logger, plan atc.TaskPlan, id event.OriginID) exec.TaskDelegate {
	return &executionDelegate{
		logger: logger,

		id:       id,
		plan:     plan,
		delegate: delegate,
	}
}

func (delegate *delegate) Finish(logger lager.Logger, err error, succeeded exec.Success, aborted bool) {
	if aborted {
		delegate.saveStatus(logger, atc.StatusAborted)

		logger.Info("aborted")
	} else if err != nil {
		delegate.saveStatus(logger, atc.StatusErrored)

		logger.Info("errored", lager.Data{"error": err.Error()})
	} else if bool(succeeded) {
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

func (delegate *delegate) saveInitializeTask(logger lager.Logger, taskConfig atc.TaskConfig, origin event.Origin) {
	err := delegate.db.SaveBuildEvent(delegate.buildID, delegate.pipelineID, event.InitializeTask{
		TaskConfig: event.ShadowTaskConfig(taskConfig),
		Origin:     origin,
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-event", err)
	}
}

func (delegate *delegate) saveInitializeGet(logger lager.Logger, origin event.Origin) {
	err := delegate.db.SaveBuildEvent(delegate.buildID, delegate.pipelineID, event.InitializeGet{
		Origin: origin,
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-event", err)
	}
}

func (delegate *delegate) saveInitializePut(logger lager.Logger, origin event.Origin) {
	err := delegate.db.SaveBuildEvent(delegate.buildID, delegate.pipelineID, event.InitializePut{
		Origin: origin,
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-event", err)
	}
}

func (delegate *delegate) saveStart(logger lager.Logger, origin event.Origin) {
	err := delegate.db.SaveBuildEvent(delegate.buildID, delegate.pipelineID, event.StartTask{
		Time:   time.Now().Unix(),
		Origin: origin,
	})
	if err != nil {
		logger.Error("failed-to-save-start-event", err)
	}
}

func (delegate *delegate) saveFinish(logger lager.Logger, status exec.ExitStatus, origin event.Origin) {
	err := delegate.db.SaveBuildEvent(delegate.buildID, delegate.pipelineID, event.FinishTask{
		ExitStatus: int(status),
		Time:       time.Now().Unix(),
		Origin:     origin,
	})
	if err != nil {
		logger.Error("failed-to-save-finish-event", err)
	}
}

func (delegate *delegate) saveStatus(logger lager.Logger, status atc.BuildStatus) {
	err := delegate.db.FinishBuild(delegate.buildID, delegate.pipelineID, db.Status(status))
	if err != nil {
		logger.Error("failed-to-finish-build", err)
	}
}

func (delegate *delegate) saveErr(logger lager.Logger, errVal error, origin event.Origin) {
	err := delegate.db.SaveBuildEvent(delegate.buildID, delegate.pipelineID, event.Error{
		Message: errVal.Error(),
		Origin:  origin,
	})
	if err != nil {
		logger.Error("failed-to-save-error-event", err)
	}
}

func (delegate *delegate) saveInput(logger lager.Logger, status exec.ExitStatus, plan atc.GetPlan, info *exec.VersionInfo, origin event.Origin) {
	var version atc.Version
	var metadata []atc.MetadataField

	if info != nil && plan.PipelineID != 0 {
		savedVR, err := delegate.db.SaveBuildInput(delegate.buildID, db.BuildInput{
			Name:              plan.Name,
			VersionedResource: vrFromInput(plan, *info),
		})
		if err != nil {
			logger.Error("failed-to-save-input", err)
		}

		version = atc.Version(savedVR.Version)
		metadata = dbMetadataToATCMetadata(savedVR.Metadata)
	}

	ev := event.FinishGet{
		Origin: origin,
		Plan: event.GetPlan{
			Name:     plan.Name,
			Resource: plan.Resource,
			Type:     plan.Type,
			Version:  plan.Version,
		},
		ExitStatus:      int(status),
		FetchedVersion:  version,
		FetchedMetadata: metadata,
	}

	err := delegate.db.SaveBuildEvent(delegate.buildID, delegate.pipelineID, ev)
	if err != nil {
		logger.Error("failed-to-save-input-event", err)
	}
}

func (delegate *delegate) saveOutput(logger lager.Logger, status exec.ExitStatus, plan atc.PutPlan, info *exec.VersionInfo, origin event.Origin) {
	var version atc.Version
	var metadata []atc.MetadataField

	if info != nil {
		version = info.Version
		metadata = info.Metadata
	}

	ev := event.FinishPut{
		Origin: origin,
		Plan: event.PutPlan{
			Name:     plan.Name,
			Resource: plan.Resource,
			Type:     plan.Type,
		},
		ExitStatus:      int(status),
		CreatedVersion:  version,
		CreatedMetadata: metadata,
	}

	err := delegate.db.SaveBuildEvent(delegate.buildID, delegate.pipelineID, ev)
	if err != nil {
		logger.Error("failed-to-save-output-event", err)
	}

	if info != nil && plan.PipelineID != 0 {
		_, err = delegate.db.SaveBuildOutput(delegate.buildID, vrFromOutput(plan.PipelineID, ev), true)
		if err != nil {
			logger.Error("failed-to-save-output", err)
		}
	}
}

func (delegate *delegate) saveImplicitOutput(logger lager.Logger, plan atc.GetPlan, info exec.VersionInfo) {
	if plan.PipelineID == 0 {
		return
	}

	metadata := make([]db.MetadataField, len(info.Metadata))
	for i, md := range info.Metadata {
		metadata[i] = db.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	_, err := delegate.db.SaveBuildOutput(delegate.buildID, db.VersionedResource{
		PipelineID: plan.PipelineID,
		Resource:   plan.Resource,
		Type:       plan.Type,
		Version:    db.Version(info.Version),
		Metadata:   metadata,
	}, false)
	if err != nil {
		logger.Error("failed-to-save", err)
		return
	}

	logger.Info("saved", lager.Data{"resource": plan.Resource})
}

func (delegate *delegate) eventWriter(origin event.Origin) io.Writer {
	return &dbEventWriter{
		db:         delegate.db,
		buildID:    delegate.buildID,
		pipelineID: delegate.pipelineID,
		origin:     origin,
	}
}

type inputDelegate struct {
	logger lager.Logger

	plan     atc.GetPlan
	id       event.OriginID
	delegate *delegate
}

func (input *inputDelegate) Initializing() {
	input.delegate.saveInitializeGet(input.logger, event.Origin{ID: input.id})
}

func (input *inputDelegate) Completed(status exec.ExitStatus, info *exec.VersionInfo) {
	input.delegate.saveInput(input.logger, status, input.plan, info, event.Origin{
		ID: input.id,
	})

	if info != nil {
		input.delegate.registerImplicitOutput(input.plan.Resource, implicitOutput{input.plan, *info})
	}

	input.logger.Info("finished", lager.Data{"version-info": info})
}

func (input *inputDelegate) Failed(err error) {
	input.delegate.saveErr(input.logger, err, event.Origin{
		ID: input.id,
	})

	input.logger.Info("errored", lager.Data{"error": err.Error()})
}

func (input *inputDelegate) ImageVersionDetermined(identifier worker.VolumeIdentifier) error {
	return input.delegate.db.SaveImageResourceVersion(input.delegate.buildID, atc.PlanID(input.id), *identifier.ResourceCache)
}

func (input *inputDelegate) FindContainersByDescriptors(container db.Container) ([]db.SavedContainer, error) {
	return input.delegate.db.FindContainersByDescriptors(container)
}

func (input *inputDelegate) Stdout() io.Writer {
	return input.delegate.eventWriter(event.Origin{
		Source: event.OriginSourceStdout,
		ID:     input.id,
	})
}

func (input *inputDelegate) Stderr() io.Writer {
	return input.delegate.eventWriter(event.Origin{
		Source: event.OriginSourceStderr,
		ID:     input.id,
	})
}

type outputDelegate struct {
	logger lager.Logger

	plan atc.PutPlan
	id   event.OriginID

	delegate *delegate
	hook     string
}

func (output *outputDelegate) Initializing() {
	output.delegate.saveInitializePut(output.logger, event.Origin{ID: output.id})
}

func (output *outputDelegate) Completed(status exec.ExitStatus, info *exec.VersionInfo) {
	output.delegate.unregisterImplicitOutput(output.plan.Resource)
	output.delegate.saveOutput(output.logger, status, output.plan, info, event.Origin{
		ID: output.id,
	})

	output.logger.Info("finished", lager.Data{"version-info": info})
}

func (output *outputDelegate) Failed(err error) {
	output.delegate.saveErr(output.logger, err, event.Origin{
		ID: output.id,
	})

	output.logger.Info("errored", lager.Data{"error": err.Error()})
}

func (output *outputDelegate) FindContainersByDescriptors(container db.Container) ([]db.SavedContainer, error) {
	return output.delegate.db.FindContainersByDescriptors(container)
}

func (output *outputDelegate) ImageVersionDetermined(identifier worker.VolumeIdentifier) error {
	return output.delegate.db.SaveImageResourceVersion(output.delegate.buildID, atc.PlanID(output.id), *identifier.ResourceCache)
}

func (output *outputDelegate) Stdout() io.Writer {
	return output.delegate.eventWriter(event.Origin{
		Source: event.OriginSourceStdout,
		ID:     output.id,
	})
}

func (output *outputDelegate) Stderr() io.Writer {
	return output.delegate.eventWriter(event.Origin{
		Source: event.OriginSourceStderr,
		ID:     output.id,
	})
}

type executionDelegate struct {
	logger lager.Logger

	plan atc.TaskPlan
	id   event.OriginID

	delegate *delegate

	hook string
}

func (execution *executionDelegate) Initializing(config atc.TaskConfig) {
	execution.delegate.saveInitializeTask(execution.logger, config, event.Origin{
		ID: execution.id,
	})

	execution.logger.Info("initializing")
}

func (execution *executionDelegate) Started() {
	execution.delegate.saveStart(execution.logger, event.Origin{
		ID: execution.id,
	})

	execution.logger.Info("started")
}

func (execution *executionDelegate) Finished(status exec.ExitStatus) {
	execution.delegate.saveFinish(execution.logger, status, event.Origin{
		ID: execution.id,
	})

	execution.logger.Info("finished", lager.Data{"exit-status": status})
}

func (execution *executionDelegate) Failed(err error) {
	execution.delegate.saveErr(execution.logger, err, event.Origin{
		ID: execution.id,
	})
	execution.logger.Info("errored", lager.Data{"error": err.Error()})
}

func (execution *executionDelegate) GetBuild(buildID int) (db.Build, bool, error) {
	return execution.delegate.db.GetBuild(buildID)
}

func (execution *executionDelegate) GetLatestFinishedBuildForJob(jobName string, pipelineID int) (db.Build, bool, error) {
	return execution.delegate.db.GetLatestFinishedBuildForJob(jobName, pipelineID)
}

func (execution *executionDelegate) ImageVersionDetermined(identifier worker.VolumeIdentifier) error {
	return execution.delegate.db.SaveImageResourceVersion(execution.delegate.buildID, atc.PlanID(execution.id), *identifier.ResourceCache)
}

func (execution *executionDelegate) FindContainersByDescriptors(container db.Container) ([]db.SavedContainer, error) {
	return execution.delegate.db.FindContainersByDescriptors(container)
}

func (execution *executionDelegate) Stdout() io.Writer {
	return execution.delegate.eventWriter(event.Origin{
		Source: event.OriginSourceStdout,
		ID:     execution.id,
	})
}

func (execution *executionDelegate) Stderr() io.Writer {
	return execution.delegate.eventWriter(event.Origin{
		Source: event.OriginSourceStderr,
		ID:     execution.id,
	})
}

type dbEventWriter struct {
	buildID    int
	pipelineID int

	db EngineDB

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

	writer.db.SaveBuildEvent(writer.buildID, writer.pipelineID, event.Log{
		Payload: string(text),
		Origin:  writer.origin,
	})

	return len(data), nil
}

func vrFromInput(plan atc.GetPlan, fetchedInfo exec.VersionInfo) db.VersionedResource {
	return db.VersionedResource{
		Resource:   plan.Resource,
		PipelineID: plan.PipelineID,
		Type:       plan.Type,
		Version:    db.Version(fetchedInfo.Version),
		Metadata:   atcMetadataToDBMetadata(fetchedInfo.Metadata),
	}
}

func vrFromOutput(pipelineID int, putted event.FinishPut) db.VersionedResource {
	return db.VersionedResource{
		Resource:   putted.Plan.Resource,
		PipelineID: pipelineID,
		Type:       putted.Plan.Type,
		Version:    db.Version(putted.CreatedVersion),
		Metadata:   atcMetadataToDBMetadata(putted.CreatedMetadata),
	}
}

func dbMetadataToATCMetadata(dbm []db.MetadataField) []atc.MetadataField {
	metadata := make([]atc.MetadataField, len(dbm))
	for i, md := range dbm {
		metadata[i] = atc.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	return metadata
}

func atcMetadataToDBMetadata(atcm []atc.MetadataField) []db.MetadataField {
	metadata := make([]db.MetadataField, len(atcm))
	for i, md := range atcm {
		metadata[i] = db.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	return metadata
}
