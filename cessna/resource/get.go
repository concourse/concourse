package resource

import (
	"bytes"
	"encoding/json"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/cessna"
	"github.com/concourse/baggageclaim"
	"github.com/tedsuo/ifrit"
)

type ResourceGet struct {
	Resource
	Version atc.Version
	Params  atc.Params
}

func (r ResourceGet) Get(logger lager.Logger, worker *cessna.Worker) (baggageclaim.Volume, error) {
	parentVolume, err := r.ResourceType.RootFSVolumeFor(logger, worker)
	if err != nil {
		return nil, err
	}

	// COW of RootFS Volume
	spec := baggageclaim.VolumeSpec{
		Strategy: baggageclaim.COWStrategy{
			Parent: parentVolume,
		},
		Privileged: false,
	}
	rootFSVolume, err := worker.BaggageClaimClient().CreateVolume(logger, spec)
	if err != nil {
		return nil, err
	}

	// Empty Volume for Get
	spec = baggageclaim.VolumeSpec{
		Strategy:   baggageclaim.EmptyStrategy{},
		Privileged: false,
	}
	volumeForGet, err := worker.BaggageClaimClient().CreateVolume(logger, spec)
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
		Privileged: false,
		RootFSPath: rootFSVolume.Path(),
		BindMounts: bindMounts,
	}

	container, err := worker.GardenClient().Create(gardenSpec)
	if err != nil {
		logger.Error("failed-to-create-gardenContainer-in-garden", err)
		return nil, err
	}

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

func (r ResourceGet) RootFSVolumeFor(logger lager.Logger, worker *cessna.Worker) (baggageclaim.Volume, error) {
	return r.Get(logger, worker)
}

func (r ResourceGet) newGetCommandProcess(container garden.Container, mountPath string) (*getCommandProcess, error) {
	p := &cessna.ContainerProcess{
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
	*cessna.ContainerProcess

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
