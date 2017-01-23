package cessna

import (
	"bytes"
	"encoding/json"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/tedsuo/ifrit"
)

type ResourceCheck struct {
	Resource
	Version *atc.Version
}

func (r ResourceCheck) Check(logger lager.Logger, worker Worker) (CheckResponse, error) {
	rootFSPath, err := r.ResourceType.RootFSPathFor(logger, worker)
	if err != nil {
		return nil, err
	}

	// Turn RootFS COW into Container
	gardenSpec := garden.ContainerSpec{
		Privileged: true,
		RootFSPath: rootFSPath,
	}

	gardenContainer, err := worker.GardenClient().Create(gardenSpec)
	if err != nil {
		logger.Error("failed-to-create-gardenContainer-in-garden", err)
		return nil, err
	}

	runner, err := r.newCheckCommandProcess(gardenContainer)
	if err != nil {
		return nil, err
	}

	checking := ifrit.Invoke(runner)

	err = <-checking.Wait()
	if err != nil {
		return nil, err
	}

	return runner.Response()
}

func (r ResourceCheck) newCheckCommandProcess(container garden.Container) (*checkCommandProcess, error) {
	p := &ContainerProcess{
		Container: container,
		ProcessSpec: garden.ProcessSpec{
			Path: "/opt/resource/check",
		},
	}

	i := CheckRequest{
		Source: r.Source,
	}

	if r.Version != nil {
		i.Version = *r.Version
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

	return &checkCommandProcess{
		ContainerProcess: p,
		out:              &stdout,
		err:              &stderr,
	}, nil
}

type checkCommandProcess struct {
	*ContainerProcess

	out *bytes.Buffer
	err *bytes.Buffer
}

func (c *checkCommandProcess) Response() (CheckResponse, error) {
	var o CheckResponse

	err := json.NewDecoder(c.out).Decode(&o)
	if err != nil {
		return nil, err
	}

	return o, nil
}
