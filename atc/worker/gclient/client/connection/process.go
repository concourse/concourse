package connection

import (
	"sync"

	"code.cloudfoundry.org/garden"
)

type process struct {
	id string

	processInputStream *processStream
	done               bool
	exitStatus         int
	exitErr            error
	doneL              *sync.Cond
}

func newProcess(id string, processInputStream *processStream) *process {
	return &process{
		id:                 id,
		processInputStream: processInputStream,
		doneL:              sync.NewCond(&sync.Mutex{}),
	}
}

func (p *process) ID() string {
	return p.id
}

func (p *process) Wait() (int, error) {
	p.doneL.L.Lock()

	for !p.done {
		p.doneL.Wait()
	}

	defer p.doneL.L.Unlock()

	return p.exitStatus, p.exitErr
}

func (p *process) SetTTY(tty garden.TTYSpec) error {
	return p.processInputStream.SetTTY(tty)
}

func (p *process) Signal(signal garden.Signal) error {
	return p.processInputStream.Signal(signal)
}

func (p *process) exited(exitStatus int, err error) {
	p.doneL.L.Lock()
	p.exitStatus = exitStatus
	p.exitErr = err
	p.done = true
	p.doneL.L.Unlock()

	p.doneL.Broadcast()
}
