package runtimetest

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/runtime"
)

type ProcessDefinition struct {
	Spec runtime.ProcessSpec
	Stub ProcessStub
}

type Container struct {
	ProcessDefs  []ProcessDefinition
	Props        map[string]string
	DBContainer_ *dbfakes.FakeCreatedContainer

	mtx       *sync.Mutex
	processes []*Process
}

func NewContainer() *Container {
	dbContainer := new(dbfakes.FakeCreatedContainer)
	return &Container{
		Props:        make(map[string]string),
		DBContainer_: dbContainer,
		mtx:          new(sync.Mutex),
	}
}

func (c *Container) WithProcess(spec runtime.ProcessSpec, stub ProcessStub) *Container {
	c2 := *c
	c2.ProcessDefs = make([]ProcessDefinition, len(c.ProcessDefs)+1)
	copy(c2.ProcessDefs, c.ProcessDefs)
	c2.ProcessDefs[len(c2.ProcessDefs)-1] = ProcessDefinition{
		Spec: spec,
		Stub: stub,
	}
	return &c2
}

func (c *Container) Run(ctx context.Context, spec runtime.ProcessSpec, io runtime.ProcessIO) (runtime.Process, error) {
	for i, pd := range c.ProcessDefs {
		if reflect.DeepEqual(pd.Spec, spec) {
			// remove current ProcessDefinition
			c.ProcessDefs = append(c.ProcessDefs[:i], c.ProcessDefs[i+1:]...)

			// setup a new process
			p := &Process{Spec: pd.Spec, ProcessStub: pd.Stub}
			p.addIO(io)

			c.mtx.Lock()
			c.processes = append(c.processes, p)
			c.mtx.Unlock()
			return p, nil
		}
	}
	return nil, fmt.Errorf("must setup a ProcessStub for process %q (%+v)", spec.ID, spec)
}

func (c *Container) RunningProcesses() []*Process {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	return c.processes
}

func (c *Container) Attach(ctx context.Context, id string, io runtime.ProcessIO) (runtime.Process, error) {
	for _, p := range c.RunningProcesses() {
		if p.Spec.ID == id {
			if !p.Attachable {
				return nil, fmt.Errorf("cannot attach to process %q because Attachable was not set to true in the ProcessStub", id)
			}
			p.addIO(io)
			return p, nil
		}
	}
	return nil, fmt.Errorf("must setup a ProcessStub for process %q", id)
}

func (c *Container) Properties() (map[string]string, error) {
	return c.Props, nil
}

func (c *Container) SetProperty(name string, value string) error {
	c.Props = cloneProps(c.Props)
	c.Props[name] = value
	return nil
}

func (c *Container) DBContainer() db.CreatedContainer {
	return c.DBContainer_
}

func cloneProps(m map[string]string) map[string]string {
	m2 := make(map[string]string, len(m))
	for k, v := range m {
		m2[k] = v
	}
	return m2
}
