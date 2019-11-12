package backend

import (
	"code.cloudfoundry.org/garden"
)

type Process struct{}

var _ garden.Process = (*Process)(nil)

func (p *Process) ID() (id string)                           { return }
func (p *Process) Wait() (somethingIDontKnow int, err error) { return }
func (p *Process) SetTTY(spec garden.TTYSpec) (err error)    { return }
func (p *Process) Signal(signal garden.Signal) (err error)   { return }
