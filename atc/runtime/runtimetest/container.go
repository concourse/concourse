package runtimetest

import (
	"context"
	"fmt"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/runtime"
)

type Container struct {
	Processes    map[string]ProcessStub
	Props        map[string]string
	DBContainer_ *dbfakes.FakeCreatedContainer
}

func NewContainer() Container {
	dbContainer := new(dbfakes.FakeCreatedContainer)
	return Container{
		Processes:    make(map[string]ProcessStub),
		Props:        make(map[string]string),
		DBContainer_: dbContainer,
	}
}

func (c Container) WithProcess(spec runtime.ProcessSpec, stub ProcessStub) Container {
	stubs := cloneProcs(c.Processes)
	stubs[spec.ID] = stub
	return Container{
		Processes: stubs,
		Props:     cloneProps(c.Props),
	}
}

func (c Container) Run(ctx context.Context, spec runtime.ProcessSpec, io runtime.ProcessIO) (runtime.Process, error) {
	p, ok := c.Processes[spec.ID]
	if !ok {
		return nil, fmt.Errorf("must setup a ProcessStub for process %q (%+v)", spec.ID, spec)
	}
	return &Process{id: spec.ID, ProcessStub: p, TTY: spec.TTY, io: io}, nil
}

func (c Container) Attach(ctx context.Context, id string, io runtime.ProcessIO) (runtime.Process, error) {
	p, ok := c.Processes[id]
	if !ok {
		return nil, fmt.Errorf("must setup a ProcessStub for process %q", id)
	}
	if p.Attachable {
		return &Process{ProcessStub: p, io: io}, nil
	}
	return nil, fmt.Errorf("cannot attach to process %q because Attachable was not set to true in the ProcessStub", id)
}

func (c Container) Properties() (map[string]string, error) {
	return c.Props, nil
}

func (c Container) SetProperty(name string, value string) error {
	c.Props[name] = value
	return nil
}

func (c Container) DBContainer() db.CreatedContainer {
	return c.DBContainer_
}

func cloneProcs(m map[string]ProcessStub) map[string]ProcessStub {
	m2 := make(map[string]ProcessStub, len(m))
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
