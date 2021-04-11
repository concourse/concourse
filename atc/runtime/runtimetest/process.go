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

func (p Process) ID() string {
	return p.Spec.ID
}

func (p *Process) Wait(ctx context.Context) (runtime.ProcessResult, error) {
	if p.Err != "" {
		return runtime.ProcessResult{}, errors.New(p.Err)
	}
	if p.Stderr != "" {
		fmt.Fprint(p.stderr(), p.Stderr)
	}
	if p.ExitStatus != 0 {
		return runtime.ProcessResult{ExitStatus: p.ExitStatus}, nil
	}
	json.NewEncoder(p.stdout()).Encode(p.Output)
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

func (p *Process) stdout() io.Writer {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	var writers []io.Writer
	for _, io := range p.io {
		writers = append(writers, io.Stdout)
	}

	return io.MultiWriter(writers...)
}

func (p *Process) stderr() io.Writer {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	var writers []io.Writer
	for _, io := range p.io {
		writers = append(writers, io.Stderr)
	}

	return io.MultiWriter(writers...)
}
