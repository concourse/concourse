package runtimetest

import (
	"context"
	"fmt"

	"github.com/concourse/concourse/atc/runtime"
)

type Container struct {
	Processes map[int]ProcessStub
	Props     map[string]string
}

func NewContainer() Container {
	return Container{
		Processes: make(map[int]ProcessStub),
		Props:     make(map[string]string),
	}
}

func (c Container) WithProcess(spec runtime.ProcessSpec, stub ProcessStub) Container {
	stubs := cloneProcs(c.Processes)
	stubs[spec.ID()] = stub
	return Container{
		Processes: stubs,
		Props:     cloneProps(c.Props),
	}
}

func (c Container) Run(ctx context.Context, spec runtime.ProcessSpec, io runtime.ProcessIO) (runtime.Process, error) {
	p, ok := c.Processes[spec.ID()]
	if !ok {
		return nil, fmt.Errorf("must setup a ProcessStub for process %+v", spec)
	}
	return Process{ProcessStub: p, io: io}, nil
}

func (c Container) Attach(ctx context.Context, spec runtime.ProcessSpec, io runtime.ProcessIO) (runtime.Process, error) {
	p, ok := c.Processes[spec.ID()]
	if !ok {
		return nil, fmt.Errorf("must setup a ProcessStub for process %+v", spec)
	}
	if p.Attachable {
		return Process{ProcessStub: p, io: io}, nil
	}
	return nil, fmt.Errorf("cannot attach to process %+v because Attachable was not set to true in the ProcessStub", spec)
}

func (c Container) Properties() (map[string]string, error) {
	return c.Props, nil
}

func (c Container) SetProperty(name string, value string) error {
	c.Props[name] = value
	return nil
}

func cloneProcs(m map[int]ProcessStub) map[int]ProcessStub {
	m2 := make(map[int]ProcessStub, len(m))
	for k, v := range m {
		m2[k] = v
	}
	return m2
}

func cloneProps(m map[string]string) map[string]string {
	m2 := make(map[string]string, len(m))
	for k, v := range m {
		m2[k] = v
	}
	return m2
}
