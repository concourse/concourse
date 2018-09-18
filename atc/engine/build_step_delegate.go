package engine

import (
	"io"
	"sync"
	"unicode/utf8"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
)

type BuildStepDelegate struct {
	build  db.Build
	planID atc.PlanID
	clock  clock.Clock
}

func NewBuildStepDelegate(
	build db.Build,
	planID atc.PlanID,
	clock clock.Clock,
) *BuildStepDelegate {
	return &BuildStepDelegate{
		build:  build,
		planID: planID,
		clock:  clock,
	}
}

func (delegate *BuildStepDelegate) ImageVersionDetermined(resourceCache db.UsedResourceCache) error {
	return delegate.build.SaveImageResourceVersion(resourceCache)
}

func (delegate *BuildStepDelegate) Stdout() io.Writer {
	return newDBEventWriter(
		delegate.build,
		event.Origin{
			Source: event.OriginSourceStdout,
			ID:     event.OriginID(delegate.planID),
		},
		delegate.clock,
	)
}

func (delegate *BuildStepDelegate) Stderr() io.Writer {
	return newDBEventWriter(
		delegate.build,
		event.Origin{
			Source: event.OriginSourceStderr,
			ID:     event.OriginID(delegate.planID),
		},
		delegate.clock,
	)
}

func (delegate *BuildStepDelegate) Errored(logger lager.Logger, message string) {
	err := delegate.build.SaveEvent(event.Error{
		Message: message,
		Origin: event.Origin{
			ID: event.OriginID(delegate.planID),
		},
	})
	if err != nil {
		logger.Error("failed-to-save-error-event", err)
	}
}

func newDBEventWriter(build db.Build, origin event.Origin, clock clock.Clock) io.Writer {
	return &dbEventWriter{
		build:  build,
		origin: origin,
		clock:  clock,
	}
}

type dbEventWriter struct {
	build db.Build

	origin event.Origin

	dangling []byte

	clock clock.Clock
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
		Time:    writer.clock.Now().Unix(),
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
