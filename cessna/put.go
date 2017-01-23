package cessna

import (
	"bytes"
	"encoding/json"

	"archive/tar"
	"os"
	"path"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/baggageclaim"
	uuid "github.com/nu7hatch/gouuid"
	"github.com/tedsuo/ifrit"
)

type ResourcePut struct {
	Resource
	Params atc.Params
}

func (r ResourcePut) Put(logger lager.Logger, worker Worker, artifacts NamedArtifacts) (OutResponse, error) {
	rootFSPath, err := r.ResourceType.RootFSPathFor(logger, worker)
	if err != nil {
		return OutResponse{}, err
	}

	// Turning artifacts into COWs
	cowArtifacts := make(NamedArtifacts)

	var spec baggageclaim.VolumeSpec
	for name, volume := range artifacts {
		spec = baggageclaim.VolumeSpec{
			Strategy: baggageclaim.COWStrategy{
				Parent: volume,
			},
			Privileged: false,
		}
		handle, err := uuid.NewV4()
		if err != nil {
			return OutResponse{}, err
		}

		v, err := worker.BaggageClaimClient().CreateVolume(logger, handle.String(), spec)
		if err != nil {
			return OutResponse{}, err
		}

		cowArtifacts[name] = v
	}

	// Create bindmounts for those COWs
	var bindMounts []garden.BindMount

	baseDirectory := "/tmp/artifacts"
	for name, volume := range cowArtifacts {
		bindMounts = append(bindMounts, garden.BindMount{
			SrcPath: volume.Path(),
			DstPath: path.Join(baseDirectory, name),
			Mode:    garden.BindMountModeRW,
		})
	}

	// Create container
	gardenSpec := garden.ContainerSpec{
		Privileged: true,
		RootFSPath: rootFSPath,
		BindMounts: bindMounts,
	}

	container, err := worker.GardenClient().Create(gardenSpec)
	if err != nil {
		logger.Error("failed-to-create-gardenContainer-in-garden", err)
		return OutResponse{}, err
	}

	// Stream fake tar into container to make sure directory exists
	// stream into baseDirectory
	emptyTar := new(bytes.Buffer)

	err = tar.NewWriter(emptyTar).Close()
	if err != nil {
		return OutResponse{}, err
	}

	err = container.StreamIn(garden.StreamInSpec{
		Path:      baseDirectory,
		TarStream: emptyTar,
	})

	// Create the PutProcess
	runner, err := r.newPutCommandProcess(container, baseDirectory)
	if err != nil {
		logger.Error("failed-to-create-get-command-process", err)
	}

	// Run the PutProcess
	putting := ifrit.Invoke(runner)

	err = <-putting.Wait()
	if err != nil {
		return OutResponse{}, err
	}

	// Parse the PutProcess output
	return runner.Response()
}

func (r ResourcePut) newPutCommandProcess(container garden.Container, artifactsDirectory string) (*putCommandProcess, error) {
	p := &ContainerProcess{
		Container: container,
		ProcessSpec: garden.ProcessSpec{
			Path: "/opt/resource/out",
			Args: []string{artifactsDirectory},
		},
	}

	i := OutRequest{
		Source: r.Source,
		Params: r.Params,
	}

	input, err := json.Marshal(i)
	if err != nil {
		return nil, err
	}

	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)

	p.ProcessIO.Stdin = bytes.NewBuffer(input)
	p.ProcessIO.Stdout = &stdout
	p.ProcessIO.Stderr = &stderr

	return &putCommandProcess{
		ContainerProcess: p,
		out:              &stdout,
		err:              &stderr,
	}, nil
}

type putCommandProcess struct {
	*ContainerProcess

	out *bytes.Buffer
	err *bytes.Buffer
}

func (g *putCommandProcess) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	err := g.ContainerProcess.Run(signals, ready)
	if err != nil {
		switch e := err.(type) {
		case ErrScriptFailed:
			e.Stderr = string(g.err.Bytes())

			err = e
		}
	}

	return err
}

func (g *putCommandProcess) Response() (OutResponse, error) {
	var r OutResponse

	err := json.NewDecoder(g.out).Decode(&r)
	if err != nil {
		return OutResponse{}, err
	}

	return r, nil
}
