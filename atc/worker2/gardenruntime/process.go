package gardenruntime

import (
	"context"
	"fmt"
	"strconv"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker/gclient"
	"github.com/hashicorp/go-multierror"
)

type Process struct {
	GardenContainer gclient.Container
	GardenProcess   garden.Process
}

func (p Process) Wait(ctx context.Context) (runtime.ProcessResult, error) {
	type result struct {
		exitStatus int
		err        error
	}
	waitResult := make(chan result, 1)

	go func() {
		exitStatus, err := p.GardenProcess.Wait()
		waitResult <- result{exitStatus: exitStatus, err: err}
	}()

	select {
	case <-ctx.Done():
		err := p.GardenContainer.Stop(false)
		// TODO: forcibly stop after timeout?
		<-waitResult
		return runtime.ProcessResult{}, multierror.Append(ctx.Err(), err)
	case r := <-waitResult:
		if r.err != nil {
			return runtime.ProcessResult{}, fmt.Errorf("wait for process completion: %w", r.err)
		}
		p.GardenContainer.SetProperty(exitStatusPropertyName, strconv.Itoa(r.exitStatus))
		return runtime.ProcessResult{ExitStatus: r.exitStatus}, nil
	}
}

func (p Process) SetTTY(tty runtime.TTYSpec) error {
	return p.GardenProcess.SetTTY(toGardenTTYSpec(tty))
}

type ExitedProcess struct {
	Result runtime.ProcessResult
}

func (p ExitedProcess) Wait(ctx context.Context) (runtime.ProcessResult, error) {
	return p.Result, nil
}

func (p ExitedProcess) SetTTY(tty runtime.TTYSpec) error {
	return nil
}
