package worker

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path"
	"strconv"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/runtime"
)

const taskProcessID = "task"
const taskExitStatusPropertyName = "concourse:exit-status"

//go:generate counterfeiter . Client

type Client interface {
	Name() string

	Worker() Worker

	RunCheckStep(
		context.Context,
		db.ContainerOwner,
		ContainerSpec,
		db.ContainerMetadata,
		runtime.ProcessSpec,
		runtime.StartingEventDelegate,
		resource.Resource,
	) (CheckResult, error)

	RunTaskStep(
		context.Context,
		db.ContainerOwner,
		ContainerSpec,
		db.ContainerMetadata,
		runtime.ProcessSpec,
		runtime.StartingEventDelegate,
	) (TaskResult, error)

	RunPutStep(
		context.Context,
		db.ContainerOwner,
		ContainerSpec,
		db.ContainerMetadata,
		runtime.ProcessSpec,
		runtime.StartingEventDelegate,
		resource.Resource,
	) (PutResult, error)

	RunGetStep(
		context.Context,
		db.ContainerOwner,
		ContainerSpec,
		db.ContainerMetadata,
		runtime.ProcessSpec,
		runtime.StartingEventDelegate,
		db.UsedResourceCache,
		resource.Resource,
	) (GetResult, error)
}

func NewClient(worker Worker) *client {
	return &client{
		worker: worker,
	}
}

type client struct {
	worker Worker
}

type TaskResult struct {
	ExitStatus   int
	VolumeMounts []VolumeMount
}

type CheckResult struct {
	Versions []atc.Version
}

type PutResult struct {
	ExitStatus    int
	VersionResult runtime.VersionResult
}

type GetResult struct {
	ExitStatus    int
	VersionResult runtime.VersionResult
	GetArtifact   runtime.GetArtifact
}

type processStatus struct {
	processStatus int
	processErr    error
}

func (client *client) Name() string {
	return client.worker.Name()
}

func (client *client) Worker() Worker {
	return client.worker
}

func (client *client) RunCheckStep(
	ctx context.Context,
	owner db.ContainerOwner,
	containerSpec ContainerSpec,
	containerMetadata db.ContainerMetadata,
	processSpec runtime.ProcessSpec,
	eventDelegate runtime.StartingEventDelegate,
	checkable resource.Resource,
) (CheckResult, error) {
	logger := lagerctx.FromContext(ctx)

	container, err := client.worker.FindOrCreateContainer(
		ctx,
		logger,
		owner,
		containerMetadata,
		containerSpec,
	)
	if err != nil {
		return CheckResult{}, err
	}

	eventDelegate.Starting(logger)

	versions, err := checkable.Check(ctx, processSpec, container)
	if err != nil {
		return CheckResult{}, fmt.Errorf("check: %w", err)
	}

	return CheckResult{Versions: versions}, nil
}

func (client *client) RunTaskStep(
	ctx context.Context,
	owner db.ContainerOwner,
	containerSpec ContainerSpec,
	metadata db.ContainerMetadata,
	processSpec runtime.ProcessSpec,
	eventDelegate runtime.StartingEventDelegate,
) (TaskResult, error) {
	logger := lagerctx.FromContext(ctx)

	container, err := client.worker.FindOrCreateContainer(
		ctx,
		logger,
		owner,
		metadata,
		containerSpec,
	)

	if err != nil {
		return TaskResult{}, err
	}

	// container already exited
	exitStatusProp, _ := container.Properties()
	code := exitStatusProp[taskExitStatusPropertyName]
	if code != "" {
		logger.Info("already-exited", lager.Data{"status": taskExitStatusPropertyName})

		status, err := strconv.Atoi(code)
		if err != nil {
			return TaskResult{}, err
		}

		return TaskResult{
			ExitStatus:   status,
			VolumeMounts: container.VolumeMounts(),
		}, err
	}

	processIO := garden.ProcessIO{
		Stdout: processSpec.StdoutWriter,
		Stderr: processSpec.StderrWriter,
	}

	// XXX(aoldershaw): why are we not using ctx?
	process, err := container.Attach(context.Background(), taskProcessID, processIO)
	if err == nil {
		logger.Info("already-running")
	} else {
		eventDelegate.Starting(logger)
		logger.Info("spawning")

		process, err = container.Run(
			context.Background(),
			garden.ProcessSpec{
				ID: taskProcessID,

				Path: processSpec.Path,
				Args: processSpec.Args,

				Dir: path.Join(metadata.WorkingDirectory, processSpec.Dir),

				// Guardian sets the default TTY window size to width: 80, height: 24,
				// which creates ANSI control sequences that do not work with other window sizes
				TTY: &garden.TTYSpec{
					WindowSize: &garden.WindowSize{Columns: 500, Rows: 500},
				},
			},
			processIO,
		)

		if err != nil {
			return TaskResult{}, err
		}
	}

	logger.Info("attached")

	exitStatusChan := make(chan processStatus)

	go func() {
		status := processStatus{}
		status.processStatus, status.processErr = process.Wait()
		exitStatusChan <- status
	}()

	select {
	case <-ctx.Done():
		err = container.Stop(false)
		if err != nil {
			logger.Error("stopping-container", err)
		}

		status := <-exitStatusChan
		err := container.UpdateExitCode(status.processStatus)
		if err != nil {
			return TaskResult{
				ExitStatus: status.processStatus,
			}, err
		}
		return TaskResult{
			ExitStatus:   status.processStatus,
			VolumeMounts: container.VolumeMounts(),
		}, ctx.Err()

	case status := <-exitStatusChan:
		err := container.UpdateExitCode(status.processStatus)
		if err != nil {
			return TaskResult{
				ExitStatus: status.processStatus,
			}, err
		}

		if status.processErr != nil {
			return TaskResult{
				ExitStatus: status.processStatus,
			}, status.processErr
		}

		err = container.SetProperty(taskExitStatusPropertyName, fmt.Sprintf("%d", status.processStatus))
		if err != nil {
			return TaskResult{
				ExitStatus: status.processStatus,
			}, err
		}
		return TaskResult{
			ExitStatus:   status.processStatus,
			VolumeMounts: container.VolumeMounts(),
		}, err
	}
}

func (client *client) RunGetStep(
	ctx context.Context,
	owner db.ContainerOwner,
	containerSpec ContainerSpec,
	containerMetadata db.ContainerMetadata,
	processSpec runtime.ProcessSpec,
	eventDelegate runtime.StartingEventDelegate,
	resourceCache db.UsedResourceCache,
	resource resource.Resource,
) (GetResult, error) {
	logger := lagerctx.FromContext(ctx)

	sign, err := resource.Signature()
	if err != nil {
		return GetResult{}, err
	}

	lockName := lockName(sign, client.worker.Name())

	// TODO: this needs to be emitted right before executing the `in` script
	eventDelegate.Starting(logger)

	getResult, _, err := client.worker.Fetch(
		ctx,
		logger,
		containerMetadata,
		client.worker,
		containerSpec,
		processSpec,
		resource,
		owner,
		resourceCache,
		lockName,
	)
	return getResult, err
}

func (client *client) RunPutStep(
	ctx context.Context,
	owner db.ContainerOwner,
	containerSpec ContainerSpec,
	metadata db.ContainerMetadata,
	spec runtime.ProcessSpec,
	eventDelegate runtime.StartingEventDelegate,
	resource resource.Resource,
) (PutResult, error) {
	logger := lagerctx.FromContext(ctx)

	container, err := client.worker.FindOrCreateContainer(
		ctx,
		logger,
		owner,
		metadata,
		containerSpec,
	)
	if err != nil {
		return PutResult{}, err
	}

	// container already exited
	exitStatusProp, err := container.Property(taskExitStatusPropertyName)
	if err == nil {
		logger.Info("already-exited", lager.Data{"status": exitStatusProp})

		status, err := strconv.Atoi(exitStatusProp)
		if err != nil {
			return PutResult{}, err
		}

		return PutResult{
			ExitStatus:    status,
			VersionResult: runtime.VersionResult{},
		}, nil
	}

	eventDelegate.Starting(logger)

	vr, err := resource.Put(ctx, spec, container)
	if err != nil {
		if failErr, ok := err.(runtime.ErrResourceScriptFailed); ok {
			return PutResult{
				ExitStatus:    failErr.ExitStatus,
				VersionResult: runtime.VersionResult{},
			}, nil
		} else {
			return PutResult{}, err
		}
	}
	return PutResult{
		ExitStatus:    0,
		VersionResult: vr,
	}, nil
}

func lockName(resourceJSON []byte, workerName string) string {
	jsonRes := append(resourceJSON, []byte(workerName)...)
	return fmt.Sprintf("%x", sha256.Sum256(jsonRes))
}
