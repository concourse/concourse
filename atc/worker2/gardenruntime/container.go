package gardenruntime

import (
	"context"
	"fmt"
	"strconv"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker/gclient"
)

const userPropertyName = "user"

const exitStatusPropertyName = "concourse:exit-status"

type Container struct {
	GardenContainer gclient.Container
}

func (c Container) Run(ctx context.Context, spec runtime.ProcessSpec, io runtime.ProcessIO) (runtime.Process, error) {
	properties, err := c.GardenContainer.Properties()
	if err != nil {
		return nil, fmt.Errorf("get properties: %w", err)
	}
	process, err := c.GardenContainer.Run(ctx, toGardenProcessSpec(spec, properties), toGardenProcessIO(io))
	if err != nil {
		return nil, fmt.Errorf("start process: %w", err)
	}

	return Process{GardenContainer: c.GardenContainer, GardenProcess: process}, nil
}

func (c Container) Attach(ctx context.Context, spec runtime.ProcessSpec, io runtime.ProcessIO) (runtime.Process, error) {
	properties, _ := c.GardenContainer.Properties()
	statusStr, ok := properties[exitStatusPropertyName]
	if ok {
		if status, err := strconv.Atoi(statusStr); err == nil {
			return ExitedProcess{Result: runtime.ProcessResult{ExitStatus: status}}, nil
		}
	}

	process, err := c.GardenContainer.Attach(ctx, strconv.Itoa(spec.ID()), toGardenProcessIO(io))
	if err != nil {
		return nil, fmt.Errorf("start process: %w", err)
	}

	return Process{GardenContainer: c.GardenContainer, GardenProcess: process}, nil
}

func (c Container) SetProperty(name string, value string) error {
	return c.GardenContainer.SetProperty(name, value)
}

func (c Container) Properties() (map[string]string, error) {
	return c.GardenContainer.Properties()
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
