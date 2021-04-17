package runtimetest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/concourse/concourse/atc/runtime"
)

type ProcessStub struct {
	Attachable bool
	Do         func(context.Context, *Process) error
	Output     interface{}
	Stderr     string
	ExitStatus int
	Err        string
}

type Process struct {
	ProcessStub
	Spec runtime.ProcessSpec

	mtx sync.Mutex
	io  []runtime.ProcessIO
}

func (p *Process) ID() string {
	return p.Spec.ID
}

func (p *Process) Wait(ctx context.Context) (runtime.ProcessResult, error) {
	if p.Do != nil {
		if err := p.Do(ctx, p); err != nil {
			return runtime.ProcessResult{}, err
		}
	}
	if p.Err != "" {
		return runtime.ProcessResult{}, errors.New(p.Err)
	}
	if p.ProcessStub.Stderr != "" {
		fmt.Fprint(p.Stderr(), p.ProcessStub.Stderr)
	}
	if p.ExitStatus != 0 {
		return runtime.ProcessResult{ExitStatus: p.ExitStatus}, nil
	}
	json.NewEncoder(p.Stdout()).Encode(p.Output)
	return runtime.ProcessResult{ExitStatus: 0}, nil
}

func (p *Process) SetTTY(tty runtime.TTYSpec) error {
	p.Spec.TTY = &tty
	return nil
}

func (p *Process) addIO(io runtime.ProcessIO) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	p.io = append(p.io, io)
}

func (p *Process) Stdin() io.Reader {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	var readers []io.Reader
	for _, io := range p.io {
		if io.Stdin != nil {
			readers = append(readers, io.Stdin)
		}
	}

	return io.MultiReader(readers...)
}

func (p *Process) Stdout() io.Writer {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	var writers []io.Writer
	for _, io := range p.io {
		if io.Stdout != nil {
			writers = append(writers, io.Stdout)
		}
	}

	return io.MultiWriter(writers...)
}

func (p *Process) Stderr() io.Writer {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	var writers []io.Writer
	for _, io := range p.io {
		if io.Stderr != nil {
			writers = append(writers, io.Stderr)
		}
	}

	return io.MultiWriter(writers...)
}
