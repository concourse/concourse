package runtimetest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
	io runtime.ProcessIO
}

func (p Process) Wait(ctx context.Context) (runtime.ProcessResult, error) {
	if p.Err != "" {
		return runtime.ProcessResult{}, errors.New(p.Err)
	}
	if p.Stderr != "" {
		fmt.Fprint(p.io.Stderr, p.Stderr)
	}
	if p.ExitStatus != 0 {
		return runtime.ProcessResult{ExitStatus: p.ExitStatus}, nil
	}
	json.NewEncoder(p.io.Stdout).Encode(p.Output)
	return runtime.ProcessResult{ExitStatus: 0}, nil
}
