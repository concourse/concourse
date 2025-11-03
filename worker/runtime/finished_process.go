//go:build linux

package runtime

import (
	"code.cloudfoundry.org/garden"
)

type finishedProcess struct {
	id         string
	exitStatus int
}

func NewFinishedProcess(id string, exitStatus int) garden.Process {
	return &finishedProcess{id: id, exitStatus: exitStatus}
}

func (p *finishedProcess) ID() string                        { return p.id }
func (p *finishedProcess) Wait() (int, error)                { return p.exitStatus, nil }
func (p *finishedProcess) SetTTY(garden.TTYSpec) error       { return nil }
func (p *finishedProcess) Signal(signal garden.Signal) error { return ErrNotImplemented }
