package engine

import (
	"io"
	"sync"
	"time"
	"unicode/utf8"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
)

type implicitOutput struct {
	plan atc.GetPlan
	info atc.VersionInfo
}

type ImplicitOutputsRepo interface {
	Register(resource string, output implicitOutput)
	Unregister(resource string)
}

//go:generate counterfeiter . BuildDelegate

type BuildDelegate interface {
	GetBuildDelegate(atc.PlanID, atc.GetPlan, *atc.VersionInfo) exec.BuildDelegate
	ExecutionDelegate(lager.Logger, atc.TaskPlan, event.OriginID) exec.TaskDelegate
	OutputDelegate(lager.Logger, atc.PutPlan, event.OriginID) exec.PutDelegate
	ImageFetchingDelegate(atc.PlanID) exec.ImageFetchingDelegate

	Finish(lager.Logger, error, exec.Success, bool)
}

//go:generate counterfeiter . BuildDelegateFactory

type BuildDelegateFactory interface {
	Delegate(db.Build) BuildDelegate
}

type buildDelegateFactory struct{}

func NewBuildDelegateFactory() BuildDelegateFactory {
	return buildDelegateFactory{}
}

func (factory buildDelegateFactory) Delegate(build db.Build) BuildDelegate {
	return newBuildDelegate(build)
}

type delegate struct {
	build               db.Build
	implicitOutputsRepo *implicitOutputsRepo
}

func newBuildDelegate(build db.Build) BuildDelegate {
	return &delegate{
		build: build,

		implicitOutputsRepo: &implicitOutputsRepo{
			outputs: make(map[string]implicitOutput),
			lock:    &sync.Mutex{},
		},
	}
}

func (delegate *delegate) GetBuildDelegate(
	planID atc.PlanID,
	plan atc.GetPlan,
	result *atc.VersionInfo,
) exec.BuildDelegate {
	return NewGetBuildDelegate(delegate.build, planID, plan, delegate.implicitOutputsRepo, result)
}

func (delegate *delegate) OutputDelegate(logger lager.Logger, plan atc.PutPlan, id event.OriginID) exec.PutDelegate {
	return &outputDelegate{
		logger: logger,

		id:                  id,
		plan:                plan,
		delegate:            delegate,
		implicitOutputsRepo: delegate.implicitOutputsRepo,
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

func (delegate *delegate) ImageFetchingDelegate(planID atc.PlanID) exec.ImageFetchingDelegate {
	return &imageFetchingDelegate{
		build:  delegate.build,
		planID: planID,
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

		for _, o := range delegate.implicitOutputsRepo.outputs {
			delegate.saveImplicitOutput(implicits.Session(o.plan.Name), o.plan, o.info)
		}

		logger.Info("succeeded")
	} else {
		delegate.saveStatus(logger, atc.StatusFailed)

		logger.Info("failed")
	}
}

func (delegate *delegate) saveInitializeTask(logger lager.Logger, taskConfig atc.TaskConfig, origin event.Origin) {
	err := delegate.build.SaveEvent(event.InitializeTask{
		TaskConfig: event.ShadowTaskConfig(taskConfig),
		Origin:     origin,
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-event", err)
	}
}

func (delegate *delegate) saveInitializePut(logger lager.Logger, origin event.Origin) {
	err := delegate.build.SaveEvent(event.InitializePut{
		Origin: origin,
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-event", err)
	}
}

func (delegate *delegate) saveStart(logger lager.Logger, origin event.Origin) {
	err := delegate.build.SaveEvent(event.StartTask{
		Time:   time.Now().Unix(),
		Origin: origin,
	})
	if err != nil {
		logger.Error("failed-to-save-start-event", err)
	}
}

func (delegate *delegate) saveFinish(logger lager.Logger, status exec.ExitStatus, origin event.Origin) {
	err := delegate.build.SaveEvent(event.FinishTask{
		ExitStatus: int(status),
		Time:       time.Now().Unix(),
		Origin:     origin,
	})
	if err != nil {
		logger.Error("failed-to-save-finish-event", err)
	}
}

func (delegate *delegate) saveStatus(logger lager.Logger, status atc.BuildStatus) {
	err := delegate.build.Finish(db.BuildStatus(status))
	if err != nil {
		logger.Error("failed-to-finish-build", err)
	}
}

func (delegate *delegate) saveErr(logger lager.Logger, errVal error, origin event.Origin) {
	err := delegate.build.SaveEvent(event.Error{
		Message: errVal.Error(),
		Origin:  origin,
	})
	if err != nil {
		logger.Error("failed-to-save-error-event", err)
	}
}

func (delegate *delegate) saveOutput(logger lager.Logger, status exec.ExitStatus, plan atc.PutPlan, info *atc.VersionInfo, origin event.Origin) {
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

	err := delegate.build.SaveEvent(ev)
	if err != nil {
		logger.Error("failed-to-save-output-event", err)
	}

	if info != nil {
		err = delegate.build.SaveOutput(vrFromOutput(ev), true)
		if err != nil {
			logger.Error("failed-to-save-output", err)
		}
	}
}

func (delegate *delegate) saveImplicitOutput(logger lager.Logger, plan atc.GetPlan, info atc.VersionInfo) {
	metadata := make([]db.ResourceMetadataField, len(info.Metadata))
	for i, md := range info.Metadata {
		metadata[i] = db.ResourceMetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	err := delegate.build.SaveOutput(db.VersionedResource{
		Resource: plan.Resource,
		Type:     plan.Type,
		Version:  db.ResourceVersion(info.Version),
		Metadata: metadata,
	}, false)
	if err != nil {
		logger.Error("failed-to-save", err)
		return
	}

	logger.Info("saved", lager.Data{"resource": plan.Resource})
}

func (delegate *delegate) eventWriter(origin event.Origin) io.Writer {
	return &dbEventWriter{
		build:  delegate.build,
		origin: origin,
	}
}

type outputDelegate struct {
	logger lager.Logger

	plan atc.PutPlan
	id   event.OriginID

	delegate            *delegate
	implicitOutputsRepo *implicitOutputsRepo
}

func (output *outputDelegate) Initializing() {
	output.delegate.saveInitializePut(output.logger, event.Origin{ID: output.id})
}

func (output *outputDelegate) Completed(status exec.ExitStatus, info *atc.VersionInfo) {
	output.implicitOutputsRepo.Unregister(output.plan.Resource)
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

func (output *outputDelegate) ImageVersionDetermined(resourceCache *db.UsedResourceCache) error {
	return output.delegate.build.SaveImageResourceVersion(resourceCache)
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

func (execution *executionDelegate) ImageVersionDetermined(resourceCache *db.UsedResourceCache) error {
	return execution.delegate.build.SaveImageResourceVersion(resourceCache)
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

func vrFromOutput(putted event.FinishPut) db.VersionedResource {
	return db.VersionedResource{
		Resource: putted.Plan.Resource,
		Type:     putted.Plan.Type,
		Version:  db.ResourceVersion(putted.CreatedVersion),
		Metadata: db.NewResourceMetadataFields(putted.CreatedMetadata),
	}
}

type imageFetchingDelegate struct {
	build  db.Build
	planID atc.PlanID
}

func (delegate *imageFetchingDelegate) ImageVersionDetermined(resourceCache *db.UsedResourceCache) error {
	return delegate.build.SaveImageResourceVersion(resourceCache)
}

func (delegate *imageFetchingDelegate) Stdout() io.Writer {
	return newDBEventWriter(
		delegate.build,
		event.Origin{
			Source: event.OriginSourceStdout,
			ID:     event.OriginID(delegate.planID),
		},
	)
}

func (delegate *imageFetchingDelegate) Stderr() io.Writer {
	return newDBEventWriter(
		delegate.build,
		event.Origin{
			Source: event.OriginSourceStderr,
			ID:     event.OriginID(delegate.planID),
		},
	)
}

func newDBEventWriter(build db.Build, origin event.Origin) io.Writer {
	return &dbEventWriter{
		build:  build,
		origin: origin,
	}
}

type dbEventWriter struct {
	build db.Build

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

	err := writer.build.SaveEvent(event.Log{
		Payload: string(text),
		Origin:  writer.origin,
	})
	if err != nil {
		return 0, err
	}

	return len(data), nil
}

type implicitOutputsRepo struct {
	outputs map[string]implicitOutput
	lock    *sync.Mutex
}

func (repo *implicitOutputsRepo) Register(resource string, output implicitOutput) {
	repo.lock.Lock()
	repo.outputs[resource] = output
	repo.lock.Unlock()
}

func (repo *implicitOutputsRepo) Unregister(resource string) {
	repo.lock.Lock()
	delete(repo.outputs, resource)
	repo.lock.Unlock()
}
