package cessna

import (
	"bytes"
	"encoding/json"

	"archive/tar"
	"os"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/baggageclaim"
	uuid "github.com/nu7hatch/gouuid"
	"github.com/tedsuo/ifrit"
)

type ResourceGet struct {
	Resource
	Version atc.Version
	Params  atc.Params
}

func (r ResourceGet) Get(logger lager.Logger, worker Worker) (baggageclaim.Volume, error) {
	rootFSPath, err := r.ResourceType.RootFSPathFor(logger, worker)
	if err != nil {
		return nil, err
	}

	// Empty Volume for Get
	spec := baggageclaim.VolumeSpec{
		Strategy:   baggageclaim.EmptyStrategy{},
		Privileged: true,
	}
	handle, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	volumeForGet, err := worker.BaggageClaimClient().CreateVolume(logger, handle.String(), spec)
	if err != nil {
		return nil, err

	}

	// Turn into Container
	mountPath := "/tmp/resource/get"
	mount := garden.BindMount{
		SrcPath: volumeForGet.Path(),
		DstPath: mountPath,
		Mode:    garden.BindMountModeRW,
	}
	bindMounts := []garden.BindMount{mount}

	gardenSpec := garden.ContainerSpec{
		Privileged: true,
		RootFSPath: rootFSPath,
		BindMounts: bindMounts,
	}

	container, err := worker.GardenClient().Create(gardenSpec)
	if err != nil {
		logger.Error("failed-to-create-gardenContainer-in-garden", err)
		return nil, err
	}

	// Stream fake tar into container to make sure directory exists
	// stream into baseDirectory
	emptyTar := new(bytes.Buffer)

	err = tar.NewWriter(emptyTar).Close()
	if err != nil {
		return nil, err
	}

	err = container.StreamIn(garden.StreamInSpec{
		Path:      mountPath,
		TarStream: emptyTar,
	})

	runner, err := r.newGetCommandProcess(container, mountPath)
	if err != nil {
		logger.Error("failed-to-create-get-command-process", err)
	}

	getting := ifrit.Invoke(runner)

	err = <-getting.Wait()
	if err != nil {
		return nil, err
	}

	return volumeForGet, nil
}

func (r ResourceGet) RootFSPathFor(logger lager.Logger, worker Worker) (string, error) {
	v, err := r.Get(logger, worker)
	if err != nil {
		return "", err
	}

	// COW of RootFS Volume
	spec := baggageclaim.VolumeSpec{
		Strategy: baggageclaim.COWStrategy{
			Parent: v,
		},
		Privileged: true,
	}
	handle, err := uuid.NewV4()
	if err != nil {
		return "", err
	}

	rootFSVolume, err := worker.BaggageClaimClient().CreateVolume(logger, handle.String(), spec)
	if err != nil {
		return "", err
	}

	return "raw://" + rootFSVolume.Path(), nil
}

func (r ResourceGet) newGetCommandProcess(container garden.Container, mountPath string) (*getCommandProcess, error) {
	p := &ContainerProcess{
		Container: container,
		ProcessSpec: garden.ProcessSpec{
			Path: "/opt/resource/in",
			Args: []string{mountPath},
		},
	}

	i := InRequest{
		Source:  r.Source,
		Params:  r.Params,
		Version: r.Version,
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

	return &getCommandProcess{
		ContainerProcess: p,
		out:              &stdout,
		err:              &stderr,
	}, nil
}

type getCommandProcess struct {
	*ContainerProcess

	out *bytes.Buffer
	err *bytes.Buffer
}

func (g *getCommandProcess) Response() (InResponse, error) {
	var r InResponse

	err := json.NewDecoder(g.out).Decode(&r)
	if err != nil {
		return InResponse{}, err
	}

	return r, nil
}

func (g *getCommandProcess) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	err := g.ContainerProcess.Run(signals, ready)
	if err != nil {
		switch e := err.(type) {
		case ErrScriptFailed:
			e.Stderr = string(g.err.Bytes())
			e.Stdout = string(g.out.Bytes())

			err = e
		}
	}

	return err
}
