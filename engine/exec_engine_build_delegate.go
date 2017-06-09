package engine

import (
	"io"
	"sync"
	"unicode/utf8"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
)

//go:generate counterfeiter . BuildDelegate

type BuildDelegate interface {
	DBBuildEventsDelegate(atc.PlanID) exec.BuildEventsDelegate
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

func (delegate *delegate) DBBuildEventsDelegate(
	planID atc.PlanID,
) exec.BuildEventsDelegate {
	return NewDBBuildEventsDelegate(delegate.build, event.Origin{ID: event.OriginID(planID)}, delegate.implicitOutputsRepo)
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

		for resourceName, o := range delegate.implicitOutputsRepo.outputs {
			delegate.saveImplicitOutput(implicits.Session(resourceName), resourceName, o.resourceType, o.info)
		}

		logger.Info("succeeded")
	} else {
		delegate.saveStatus(logger, atc.StatusFailed)

		logger.Info("failed")
	}
}

func (delegate *delegate) saveStatus(logger lager.Logger, status atc.BuildStatus) {
	err := delegate.build.Finish(db.BuildStatus(status))
	if err != nil {
		logger.Error("failed-to-finish-build", err)
	}
}

func (delegate *delegate) saveImplicitOutput(logger lager.Logger, resourceName string, resourceType string, info exec.VersionInfo) {
	metadata := make([]db.ResourceMetadataField, len(info.Metadata))
	for i, md := range info.Metadata {
		metadata[i] = db.ResourceMetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	err := delegate.build.SaveOutput(db.VersionedResource{
		Resource: resourceName,
		Type:     resourceType,
		Version:  db.ResourceVersion(info.Version),
		Metadata: metadata,
	}, false)
	if err != nil {
		logger.Error("failed-to-save", err)
		return
	}

	logger.Info("saved", lager.Data{"resource": resourceName})
}

func (delegate *delegate) eventWriter(origin event.Origin) io.Writer {
	return &dbEventWriter{
		build:  delegate.build,
		origin: origin,
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

type implicitOutput struct {
	resourceType string
	info         exec.VersionInfo
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
