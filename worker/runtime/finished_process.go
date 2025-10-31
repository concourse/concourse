//go:build linux

package runtime

import (
	"code.cloudfoundry.org/garden"
)

type finishedProcess struct {
	id       string
	exitCode int
}

func NewFinishedProcess(id string, exitCode int) garden.Process {
	return &finishedProcess{id: id, exitCode: exitCode}
}

func (p *finishedProcess) ID() string                        { return p.id }
func (p *finishedProcess) Wait() (int, error)                { return p.exitCode, nil }
func (p *finishedProcess) SetTTY(garden.TTYSpec) error       { return nil }
func (p *finishedProcess) Signal(signal garden.Signal) error { return ErrNotImplemented }
