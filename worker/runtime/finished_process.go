//go:build linux

package runtime

import (
	"code.cloudfoundry.org/garden"
)

type FinishedProcess struct {
	id         string
	exitStatus int
}

func NewFinishedProcess(id string, exitStatus int) garden.Process {
	return &FinishedProcess{id: id, exitStatus: exitStatus}
}

func (p *FinishedProcess) ID() string                        { return p.id }
func (p *FinishedProcess) Wait() (int, error)                { return p.exitStatus, nil }
func (p *FinishedProcess) SetTTY(garden.TTYSpec) error       { return nil }
func (p *FinishedProcess) Signal(signal garden.Signal) error { return ErrNotImplemented }
