package exec

import (
	"os"
	"path/filepath"

	"github.com/cloudfoundry-incubator/garden"

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

func (factory *gardenFactory) DependentGet(stepMetadata StepMetadata, sourceName SourceName, id worker.Identifier, delegate GetDelegate, config atc.ResourceConfig, tags atc.Tags, params atc.Params) StepFactory {
	return resourceStep{
		ResourceConfig: config,

		TrackerFactory: factory.trackerFactory,
		WorkerClient:   factory.workerClient,

		StepMetadata: stepMetadata,

		SourceName: sourceName,

		Session: resource.Session{
			ID:        id,
			Ephemeral: false,
		},

		Delegate: delegate,

		Type: resource.ResourceType(config.Type),
		Tags: tags,

		Action: func(r resource.Resource, s ArtifactSource, vi VersionInfo) resource.VersionedSource {
			return r.Get(resource.IOConfig{
				Stdout: delegate.Stdout(),
				Stderr: delegate.Stderr(),
			}, config.Source, params, vi.Version)
		},
	}
}

func (factory *gardenFactory) Get(
	stepMetadata StepMetadata,
	sourceName SourceName,
	id worker.Identifier,
	delegate GetDelegate,
	config atc.ResourceConfig,
	params atc.Params,
	tags atc.Tags,
	version atc.Version,
) StepFactory {
	return resourceStep{
		ResourceConfig: config,
		Version:        version,

		TrackerFactory: factory.trackerFactory,
		WorkerClient:   factory.workerClient,

		StepMetadata: stepMetadata,

		SourceName: sourceName,

		Session: resource.Session{
			ID:        id,
			Ephemeral: false,
		},

		Delegate: delegate,

		Type: resource.ResourceType(config.Type),
		Tags: tags,

		Action: func(r resource.Resource, s ArtifactSource, vi VersionInfo) resource.VersionedSource {
			return r.Get(resource.IOConfig{
				Stdout: delegate.Stdout(),
				Stderr: delegate.Stderr(),
			}, config.Source, params, version)
		},
	}
}

func (factory *gardenFactory) Put(stepMetadata StepMetadata, id worker.Identifier, delegate PutDelegate, config atc.ResourceConfig, tags atc.Tags, params atc.Params) StepFactory {
	return resourceStep{
		ResourceConfig: config,

		TrackerFactory: factory.trackerFactory,
		WorkerClient:   factory.workerClient,

		StepMetadata: stepMetadata,

		Session: resource.Session{
			ID: id,
		},

		Delegate: delegate,

		Type: resource.ResourceType(config.Type),
		Tags: tags,

		Action: func(r resource.Resource, s ArtifactSource, vi VersionInfo) resource.VersionedSource {
			return r.Put(resource.IOConfig{
				Stdout: delegate.Stdout(),
				Stderr: delegate.Stderr(),
			}, config.Source, params, resourceSource{s})
		},
	}
}

func (factory *gardenFactory) Task(sourceName SourceName, id worker.Identifier, delegate TaskDelegate, privileged Privileged, tags atc.Tags, configSource TaskConfigSource) StepFactory {
	artifactsRoot := filepath.Join("/tmp", "build", factory.uuidGenerator())

	return taskStep{
		SourceName: sourceName,

		WorkerID: id,
		Tags:     tags,

		Delegate: delegate,

		Privileged:   privileged,
		ConfigSource: configSource,

		WorkerClient: factory.workerClient,

		artifactsRoot: artifactsRoot,
	}
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
