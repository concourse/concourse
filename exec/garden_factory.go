package exec

import (
	"os"
	"path/filepath"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

//go:generate counterfeiter . TrackerFactory

type TrackerFactory interface {
	TrackerFor(worker.Client) resource.Tracker
}

type gardenFactory struct {
	workerClient   worker.Client
	trackerFactory TrackerFactory
	uuidGenerator  UUIDGenFunc
}

type UUIDGenFunc func() string

func NewGardenFactory(
	workerClient worker.Client,
	trackerFactory TrackerFactory,
	uuidGenerator UUIDGenFunc,
) Factory {
	return &gardenFactory{
		workerClient:   workerClient,
		trackerFactory: trackerFactory,
		uuidGenerator:  uuidGenerator,
	}
}

func (factory *gardenFactory) DependentGet(
	logger lager.Logger,
	stepMetadata StepMetadata,
	sourceName SourceName,
	id worker.Identifier,
	delegate GetDelegate,
	resourceConfig atc.ResourceConfig,
	tags atc.Tags,
	params atc.Params,
) StepFactory {
	return newDependentGetStep(
		logger,
		sourceName,
		factory.workerClient,
		resourceConfig,
		params,
		stepMetadata,
		resource.Session{
			ID:        id,
			Ephemeral: false,
		},
		tags,
		delegate,
		factory.trackerFactory,
	)
}

func (factory *gardenFactory) Get(
	logger lager.Logger,
	stepMetadata StepMetadata,
	sourceName SourceName,
	id worker.Identifier,
	delegate GetDelegate,
	resourceConfig atc.ResourceConfig,
	params atc.Params,
	tags atc.Tags,
	version atc.Version,
) StepFactory {
	return newGetStep(
		logger,
		sourceName,
		factory.workerClient,
		resourceConfig,
		version,
		params,
		stepMetadata,
		resource.Session{
			ID:        id,
			Ephemeral: false,
		},
		tags,
		delegate,
		factory.trackerFactory,
	)
}

func (factory *gardenFactory) Put(
	logger lager.Logger,
	stepMetadata StepMetadata,
	id worker.Identifier,
	delegate PutDelegate,
	resourceConfig atc.ResourceConfig,
	tags atc.Tags,
	params atc.Params,
) StepFactory {
	return newPutStep(
		logger,
		factory.workerClient,
		resourceConfig,
		params,
		stepMetadata,
		resource.Session{
			ID:        id,
			Ephemeral: false,
		},
		tags,
		delegate,
		factory.trackerFactory,
	)
}

func (factory *gardenFactory) Task(
	logger lager.Logger,
	sourceName SourceName,
	id worker.Identifier,
	delegate TaskDelegate,
	privileged Privileged,
	tags atc.Tags,
	configSource TaskConfigSource,
) StepFactory {
	return newTaskStep(
		sourceName,
		id,
		tags,
		delegate,
		privileged,
		configSource,
		factory.workerClient,
		filepath.Join("/tmp", "build", factory.uuidGenerator()),
	)
}

type hijackedProcess struct {
	process garden.Process
}

func (p hijackedProcess) Wait() (int, error) {
	return p.process.Wait()
}

func (p hijackedProcess) SetTTY(spec atc.HijackTTYSpec) error {
	return p.process.SetTTY(garden.TTYSpec{
		WindowSize: &garden.WindowSize{
			Columns: spec.WindowSize.Columns,
			Rows:    spec.WindowSize.Rows,
		},
	})
}

type failureReporter struct {
	Step

	ReportFailure func(error)
}

func (reporter failureReporter) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	err := reporter.Step.Run(signals, ready)
	if err != nil {
		reporter.ReportFailure(err)
	}

	return err
}

type resourceSource struct {
	ArtifactSource
}

func (source resourceSource) StreamTo(dest resource.ArtifactDestination) error {
	return source.ArtifactSource.StreamTo(resource.ArtifactDestination(dest))
}
