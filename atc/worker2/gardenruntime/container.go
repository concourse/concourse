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

const userPropertyName = "user"

const exitStatusPropertyName = "concourse:exit-status"

type Container struct {
	GardenContainer gclient.Container
	Mounts          []runtime.VolumeMount
}

func (c Container) Run(ctx context.Context, spec runtime.ProcessSpec, io runtime.ProcessIO) (runtime.ProcessResult, error) {
	properties, err := c.GardenContainer.Properties()
	if err != nil {
		return runtime.ProcessResult{}, fmt.Errorf("get properties: %w", err)
	}
	process, err := c.GardenContainer.Run(ctx, toGardenProcessSpec(spec, properties), toGardenProcessIO(io))
	if err != nil {
		return runtime.ProcessResult{}, fmt.Errorf("start process: %w", err)
	}

	return c.waitForProcessCompletion(ctx, process)
}

func (c Container) Attach(ctx context.Context, spec runtime.ProcessSpec, io runtime.ProcessIO) (runtime.ProcessResult, error) {
	properties, _ := c.GardenContainer.Properties()
	statusStr, ok := properties[exitStatusPropertyName]
	if ok {
		if status, err := strconv.Atoi(statusStr); err == nil {
			return runtime.ProcessResult{ExitStatus: status}, nil
		}
	}

	process, err := c.GardenContainer.Attach(ctx, strconv.Itoa(spec.ID()), toGardenProcessIO(io))
	if err != nil {
		return runtime.ProcessResult{}, fmt.Errorf("start process: %w", err)
	}

	return c.waitForProcessCompletion(ctx, process)
}

func (c Container) waitForProcessCompletion(ctx context.Context, process garden.Process) (runtime.ProcessResult, error) {
	type result struct {
		exitStatus int
		err        error
	}
	waitResult := make(chan result, 1)

	go func() {
		exitStatus, err := process.Wait()
		waitResult <- result{exitStatus: exitStatus, err: err}
	}()

	select {
	case <-ctx.Done():
		err := c.GardenContainer.Stop(false)
		// TODO: forcibly stop after timeout?
		<-waitResult
		return runtime.ProcessResult{}, multierror.Append(ctx.Err(), err)
	case r := <-waitResult:
		if r.err != nil {
			return runtime.ProcessResult{}, fmt.Errorf("wait for process completion: %w", r.err)
		}
		// TODO: for resources, we also want to store the stdout result. this
		// should happen externally, though
		c.GardenContainer.SetProperty(exitStatusPropertyName, strconv.Itoa(r.exitStatus))
		return runtime.ProcessResult{ExitStatus: r.exitStatus}, nil
	}
}

func (c Container) VolumeMounts() []runtime.VolumeMount {
	return c.Mounts
}

func toGardenProcessSpec(spec runtime.ProcessSpec, properties garden.Properties) garden.ProcessSpec {
	user := spec.User
	if user == "" {
		user = properties[userPropertyName]
	}
	return garden.ProcessSpec{
		ID:   strconv.Itoa(spec.ID()),
		Path: spec.Path,
		Args: spec.Args,
		Dir:  spec.Dir,
		User: user,

		// Guardian sets the default TTY window size to width: 80, height: 24,
		// which creates ANSI control sequences that do not work with other window sizes
		TTY: &garden.TTYSpec{
			WindowSize: &garden.WindowSize{Columns: 500, Rows: 500},
		},
	}
}

func toGardenProcessIO(io runtime.ProcessIO) garden.ProcessIO {
	return garden.ProcessIO{
		Stdin:  io.Stdin,
		Stdout: io.Stdout,
		Stderr: io.Stderr,
	}
}
