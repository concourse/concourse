package runtimetest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/concourse/concourse/atc/runtime"
)

type Container struct {
	ProcessStubs map[int]ProcessStub
	Props        map[string]string
}

type ProcessStub struct {
	Attachable bool
	Output     interface{}
	Stderr     string
	ExitStatus int
	Err        string
}

func NewContainer() Container {
	return Container{
		ProcessStubs: make(map[int]ProcessStub),
		Props:        make(map[string]string),
	}
}

func (c Container) WithProcess(spec runtime.ProcessSpec, stub ProcessStub) Container {
	stubs := cloneProcs(c.ProcessStubs)
	stubs[spec.ID()] = stub
	return Container{
		ProcessStubs: stubs,
		Props:        cloneProps(c.Props),
	}
}

func (c Container) Run(ctx context.Context, spec runtime.ProcessSpec, io runtime.ProcessIO) (runtime.ProcessResult, error) {
	p, ok := c.ProcessStubs[spec.ID()]
	if !ok {
		return runtime.ProcessResult{}, fmt.Errorf("must setup a ProcessStub for process %+v", spec)
	}
	if p.Err != "" {
		return runtime.ProcessResult{}, errors.New(p.Err)
	}
	if p.Stderr != "" {
		fmt.Fprint(io.Stderr, p.Stderr)
	}
	if p.ExitStatus != 0 {
		return runtime.ProcessResult{ExitStatus: p.ExitStatus}, nil
	}
	json.NewEncoder(io.Stdout).Encode(p.Output)
	return runtime.ProcessResult{ExitStatus: 0}, nil
}

func (c Container) Attach(ctx context.Context, spec runtime.ProcessSpec, io runtime.ProcessIO) (runtime.ProcessResult, error) {
	p := c.ProcessStubs[spec.ID()]
	if p.Attachable {
		return c.Run(ctx, spec, io)
	}
	return runtime.ProcessResult{}, fmt.Errorf("cannot attach to process %+v because Attachable was not set to true in the ProcessStub", spec)
}

func (c Container) Properties() (map[string]string, error) {
	return c.Props, nil
}

func (c Container) SetProperty(name string, value string) error {
	c.Props[name] = value
	return nil
}

func (c Container) VolumeMounts() []runtime.VolumeMount {
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
