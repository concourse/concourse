package exec

import (
	garden "github.com/cloudfoundry-incubator/garden/api"

	"github.com/concourse/atc"
	"github.com/concourse/atc/exec/resource"
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

func (factory *gardenFactory) Get(sessionID SessionID, ioConfig IOConfig, config atc.ResourceConfig, params atc.Params, version atc.Version) Step {
	return resourceStep{
		SessionID: resource.SessionID(sessionID),

		Tracker: factory.resourceTracker,
		Type:    resource.ResourceType(config.Type),

		Action: func(r resource.Resource, s ArtifactSource) resource.VersionedSource {
			return r.Get(resource.IOConfig{
				Stdout: ioConfig.Stdout,
				Stderr: ioConfig.Stderr,
			}, config.Source, params, version)
		},
	}
}

func (factory *gardenFactory) Put(sessionID SessionID, ioConfig IOConfig, config atc.ResourceConfig, params atc.Params) Step {
	return resourceStep{
		SessionID: resource.SessionID(sessionID),

		Tracker: factory.resourceTracker,
		Type:    resource.ResourceType(config.Type),

		Action: func(r resource.Resource, s ArtifactSource) resource.VersionedSource {
			return r.Put(resource.IOConfig{
				Stdout: ioConfig.Stdout,
				Stderr: ioConfig.Stderr,
			}, config.Source, params, resourceSource{s})
		},
	}
}

func (factory *gardenFactory) Execute(sessionID SessionID, ioConfig IOConfig, privileged Privileged, configSource BuildConfigSource) Step {
	return executeStep{
		SessionID: sessionID,

		IOConfig: ioConfig,

		Privileged:   privileged,
		ConfigSource: configSource,

		WorkerClient: factory.workerClient,
	}
}

func (factory *gardenFactory) Hijack(sessionID SessionID, ioConfig IOConfig, spec atc.HijackProcessSpec) (HijackedProcess, error) {
	container, err := factory.workerClient.Lookup(string(sessionID))
	if err != nil {
		return nil, err
	}

	process, err := container.Run(convertProcessSpec(spec), garden.ProcessIO{
		Stdin:  ioConfig.Stdin,
		Stdout: ioConfig.Stdout,
		Stderr: ioConfig.Stderr,
	})
	if err != nil {
		return nil, err
	}

	return hijackedProcess{process}, nil
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
