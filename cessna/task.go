package cessna

import (
	"bytes"

	"archive/tar"
	"path"

	"os"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim"
	uuid "github.com/nu7hatch/gouuid"
	"github.com/tedsuo/ifrit"
)

type Task struct {
	RootFSGenerator RootFSable
	Path            string
	Args            []string
	Env             []string
	Dir             string
	User            string
	Privileged      bool
}

type TaskResponse struct {
}

func (t *Task) Run(logger lager.Logger, worker Worker, inputs NamedArtifacts, outputs NamedArtifacts) error {
	rootFSPath, err := t.RootFSGenerator.RootFSPathFor(logger, worker)
	cowArtifacts := make(NamedArtifacts)

	var spec baggageclaim.VolumeSpec
	for name, volume := range inputs {
		spec = baggageclaim.VolumeSpec{
			Strategy: baggageclaim.COWStrategy{
				Parent: volume,
			},
			Privileged: t.Privileged,
		}
		handle, err := uuid.NewV4()
		if err != nil {
			return err
		}

		v, err := worker.BaggageClaimClient().CreateVolume(logger, handle.String(), spec)
		if err != nil {
			return err
		}

		cowArtifacts[name] = v
	}

	// Create bindmounts for those COWs
	var bindMounts []garden.BindMount

	workingDir := "/tmp/build/"

	for n, v := range cowArtifacts {
		bindMounts = append(bindMounts, garden.BindMount{
			SrcPath: v.Path(),
			DstPath: path.Join(workingDir, n),
			Mode:    garden.BindMountModeRW,
		})
	}

	for n, v := range outputs {
		bindMounts = append(bindMounts, garden.BindMount{
			SrcPath: v.Path(),
			DstPath: path.Join(workingDir, n),
			Mode:    garden.BindMountModeRW,
		})
	}

	// Create container
	gardenSpec := garden.ContainerSpec{
		Privileged: t.Privileged,
		RootFSPath: rootFSPath,
		BindMounts: bindMounts,
	}

	container, err := worker.GardenClient().Create(gardenSpec)
	if err != nil {
		logger.Error("failed-to-create-gardenContainer-in-garden", err)
		return err
	}

	// Stream fake tar into container to make sure directory exists
	// stream into baseDirectory
	emptyTar := new(bytes.Buffer)

	err = tar.NewWriter(emptyTar).Close()
	if err != nil {
		return err
	}

	err = container.StreamIn(garden.StreamInSpec{
		Path:      workingDir,
		TarStream: emptyTar,
	})

	r := t.RunnerFor(logger, container, workingDir)

	task := ifrit.Invoke(r)

	return <-task.Wait()
}

func (t *Task) RunnerFor(logger lager.Logger, container garden.Container, workingDir string) *taskContainerProcess {
	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)

	spec := garden.ProcessSpec{
		Path: t.Path,
		Args: t.Args,
		Env:  t.Env,
		Dir:  path.Join(workingDir, t.Dir),
		User: t.User,
	}

	io := garden.ProcessIO{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
	}

	return &taskContainerProcess{
		ContainerProcess: ContainerProcess{
			Container:   container,
			ProcessSpec: spec,
			ProcessIO:   io,
		},
		out: &stdout,
		err: &stderr,
	}

}

type taskContainerProcess struct {
	ContainerProcess

	out *bytes.Buffer
	err *bytes.Buffer
}

func (t *taskContainerProcess) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	err := t.ContainerProcess.Run(signals, ready)

	switch e := err.(type) {
	case ErrScriptFailed:
		e.Stderr = string(t.err.Bytes())
		return e
	}

	return err
}
