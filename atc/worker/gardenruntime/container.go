package gardenruntime

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagerctx"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker/gardenruntime/gclient"
)

const userPropertyName = "user"

const exitStatusPropertyName = "concourse:exit-status"

type Container struct {
	DBContainer_    db.CreatedContainer
	GardenContainer gclient.Container
}

func (worker *Worker) LookupContainer(ctx context.Context, handle string) (runtime.Container, bool, error) {
	logger := lagerctx.FromContext(ctx).Session("lookup-container", lager.Data{"handle": handle, "worker": worker.Name()})

	_, createdContainer, err := worker.dbWorker.FindContainer(db.NewFixedHandleContainerOwner(handle))
	if err != nil {
		logger.Error("failed-to-lookup-container-in-db", err)
		return Container{}, false, err
	}

	if createdContainer == nil {
		return Container{}, false, nil
	}

	gardenContainer, err := worker.gardenClient.Lookup(handle)
	if err != nil {
		if errors.As(err, &garden.ContainerNotFoundError{}) {
			logger.Debug("garden-container-not-found")
			return Container{}, false, nil
		}
		logger.Error("failed-to-lookup-garden-container", err)
		return Container{}, false, err
	}

	return Container{GardenContainer: gardenContainer, DBContainer_: createdContainer}, true, nil
}

func (c Container) Run(_ context.Context, process_spec runtime.ProcessSpec, io runtime.ProcessIO) (runtime.Process, error) {
	properties, err := c.GardenContainer.Properties()
	if err != nil {
		return nil, fmt.Errorf("get properties: %w", err)
	}
	// Using context.Background() since GardenContainer.Run uses the passed in
	// ctx to stream the process output. Since we send a SIGTERM on ctx
	// cancellation, using the same ctx for streaming results in those logs not
	// showing up.
	process, err := c.GardenContainer.Run(context.Background(), toGardenProcessSpec(process_spec, properties), toGardenProcessIO(io))
	if err != nil {
		var exeNotFound garden.ExecutableNotFoundError
		if errors.As(err, &exeNotFound) {
			return nil, runtime.ExecutableNotFoundError{Message: exeNotFound.Message}
		}
		return nil, fmt.Errorf("start process: %w", err)
	}

	return Process{GardenContainer: c.GardenContainer, GardenProcess: process}, nil
}

func (c Container) Attach(_ context.Context, id string, io runtime.ProcessIO) (runtime.Process, error) {
	properties, _ := c.GardenContainer.Properties()
	statusStr, ok := properties[exitStatusPropertyName]
	if ok {
		if status, err := strconv.Atoi(statusStr); err == nil {
			return ExitedProcess{id: id, Result: runtime.ProcessResult{ExitStatus: status}}, nil
		}
	}

	// Using context.Background() since GardenContainer.Attach uses the passed
	// in ctx to stream the process output. Since we send a SIGTERM on ctx
	// cancellation, using the same ctx for streaming results in those logs not
	// showing up.
	process, err := c.GardenContainer.Attach(context.Background(), id, toGardenProcessIO(io))
	if err != nil {
		return nil, fmt.Errorf("attach to process: %w", err)
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
	var tty *garden.TTYSpec
	if spec.TTY != nil {
		spec := toGardenTTYSpec(*spec.TTY)
		tty = &spec
	}
	return garden.ProcessSpec{
		ID:   spec.ID,
		Path: spec.Path,
		Args: spec.Args,
		Env:  spec.Env,
		Dir:  spec.Dir,
		User: user,
		TTY:  tty,
	}
}

func toGardenTTYSpec(tty runtime.TTYSpec) garden.TTYSpec {
	return garden.TTYSpec{
		WindowSize: &garden.WindowSize{
			Columns: tty.WindowSize.Columns,
			Rows:    tty.WindowSize.Rows,
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

func (c Container) DBContainer() db.CreatedContainer {
	return c.DBContainer_
}
