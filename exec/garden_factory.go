package exec

import (
	"os"

	"github.com/cloudfoundry-incubator/garden"

	"github.com/concourse/atc"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

type gardenFactory struct {
	workerClient    worker.Client
	resourceTracker resource.Tracker
}

func NewGardenFactory(
	workerClient worker.Client,
	resourceTracker resource.Tracker,
) Factory {
	return &gardenFactory{
		workerClient:    workerClient,
		resourceTracker: resourceTracker,
	}
}

func (factory *gardenFactory) Get(sourceName SourceName, id worker.Identifier, delegate GetDelegate, config atc.ResourceConfig, params atc.Params, version atc.Version) StepFactory {
	return resourceStep{
		SourceName: sourceName,

		Session: resource.Session{
			ID:        id,
			Ephemeral: false,
		},

		Delegate: delegate,

		Tracker: factory.resourceTracker,
		Type:    resource.ResourceType(config.Type),

		Action: func(r resource.Resource, s ArtifactSource) resource.VersionedSource {
			return r.Get(resource.IOConfig{
				Stdout: delegate.Stdout(),
				Stderr: delegate.Stderr(),
			}, config.Source, params, version)
		},
	}
}

func (factory *gardenFactory) Put(id worker.Identifier, delegate PutDelegate, config atc.ResourceConfig, params atc.Params) StepFactory {
	return resourceStep{
		Session: resource.Session{
			ID: id,
		},

		Delegate: delegate,

		Tracker: factory.resourceTracker,
		Type:    resource.ResourceType(config.Type),

		Action: func(r resource.Resource, s ArtifactSource) resource.VersionedSource {
			return r.Put(resource.IOConfig{
				Stdout: delegate.Stdout(),
				Stderr: delegate.Stderr(),
			}, config.Source, params, resourceSource{s})
		},
	}
}

func (factory *gardenFactory) Task(sourceName SourceName, id worker.Identifier, delegate TaskDelegate, privileged Privileged, configSource TaskConfigSource) StepFactory {
	return taskStep{
		SourceName: sourceName,

		WorkerID: id,

		Delegate: delegate,

		Privileged:   privileged,
		ConfigSource: configSource,

		WorkerClient: factory.workerClient,
	}
}

func convertProcessSpec(spec atc.HijackProcessSpec) garden.ProcessSpec {
	var tty *garden.TTYSpec
	if spec.TTY != nil {
		tty = &garden.TTYSpec{
			WindowSize: &garden.WindowSize{
				Columns: spec.TTY.WindowSize.Columns,
				Rows:    spec.TTY.WindowSize.Rows,
			},
		}
	}

	return garden.ProcessSpec{
		Path: spec.Path,
		Args: spec.Args,
		Env:  spec.Env,
		Dir:  spec.Dir,

		Privileged: spec.Privileged,
		User:       spec.User,

		TTY: tty,
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
