package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker/gclient"
)

var ErrMissingVolume = errors.New("volume mounted to container is missing")

//go:generate counterfeiter . Container

type Container interface {
	gclient.Container
	runtime.Runner

	// TODO: get rid of this, its not used anywhere
	Destroy() error

	VolumeMounts() []VolumeMount

	// TODO: get rid of this, its not used anywhere
	WorkerName() string

	MarkAsHijacked() error
}

type gardenWorkerContainer struct {
	gclient.Container
	dbContainer db.CreatedContainer
	dbVolumes   []db.CreatedVolume

	gardenClient gclient.Client

	volumeMounts []VolumeMount

	user       string
	workerName string
}

func newGardenWorkerContainer(
	logger lager.Logger,
	container gclient.Container,
	dbContainer db.CreatedContainer,
	dbContainerVolumes []db.CreatedVolume,
	gardenClient gclient.Client,
	volumeClient VolumeClient,
	workerName string,
) (Container, error) {
	logger = logger.WithData(
		lager.Data{
			"container": container.Handle(),
			"worker":    workerName,
		},
	)

	workerContainer := &gardenWorkerContainer{
		Container:   container,
		dbContainer: dbContainer,
		dbVolumes:   dbContainerVolumes,

		gardenClient: gardenClient,

		workerName: workerName,
	}

	err := workerContainer.initializeVolumes(logger, volumeClient)
	if err != nil {
		return nil, err
	}

	properties, err := workerContainer.Properties()
	if err != nil {
		return nil, err
	}

	if properties["user"] != "" {
		workerContainer.user = properties["user"]
	} else {
		workerContainer.user = "root"
	}

	return workerContainer, nil
}

func (container *gardenWorkerContainer) Destroy() error {
	return container.gardenClient.Destroy(container.Handle())
}

func (container *gardenWorkerContainer) WorkerName() string {
	return container.workerName
}

func (container *gardenWorkerContainer) MarkAsHijacked() error {
	return container.dbContainer.MarkAsHijacked()
}

func (container *gardenWorkerContainer) Run(ctx context.Context, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	spec.User = container.user
	return container.Container.Run(ctx, spec, io)
}

func (container *gardenWorkerContainer) VolumeMounts() []VolumeMount {
	return container.volumeMounts
}

func (container *gardenWorkerContainer) initializeVolumes(
	logger lager.Logger,
	volumeClient VolumeClient,
) error {

	volumeMounts := []VolumeMount{}

	for _, dbVolume := range container.dbVolumes {
		volumeLogger := logger.Session("volume", lager.Data{
			"handle": dbVolume.Handle(),
		})

		volume, volumeFound, err := volumeClient.LookupVolume(logger, dbVolume.Handle())
		if err != nil {
			volumeLogger.Error("failed-to-lookup-volume", err)
			return err
		}

		if !volumeFound {
			volumeLogger.Error("volume-is-missing-on-worker", ErrMissingVolume, lager.Data{"handle": dbVolume.Handle()})
			return errors.New("volume mounted to container is missing " + dbVolume.Handle() + " from worker " + container.workerName)
		}

		volumeMounts = append(volumeMounts, VolumeMount{
			Volume:    volume,
			MountPath: dbVolume.Path(),
		})
	}

	container.volumeMounts = volumeMounts

	return nil
}

// TODO (runtime/#4910): this needs to be modified to not be resource specific
// 		the stdout of the run() is expected to be of json format
//      this will break if used with task_step as it does not
//		print out json
func (container *gardenWorkerContainer) RunScript(
	ctx context.Context,
	path string,
	args []string,
	input []byte,
	output interface{},
	logDest io.Writer,
	recoverable bool,
) error {
	if recoverable {
		result, _ := container.Properties()
		code := result[runtime.ResourceResultPropertyName]
		if code != "" {
			return json.Unmarshal([]byte(code), &output)
		}
	}

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	processIO := garden.ProcessIO{
		Stdin:  bytes.NewBuffer(input),
		Stdout: stdout,
	}

	if logDest != nil {
		processIO.Stderr = logDest
	} else {
		processIO.Stderr = stderr
	}

	var process garden.Process
	var err error
	if recoverable {
		process, err = container.Attach(ctx, runtime.ResourceProcessID, processIO)
		if err != nil {
			process, err = container.Run(
				ctx,
				garden.ProcessSpec{
					ID:   runtime.ResourceProcessID,
					Path: path,
					Args: args,
				}, processIO)
			if err != nil {
				return err
			}
		}
	} else {
		process, err = container.Run(ctx, garden.ProcessSpec{
			Path: path,
			Args: args,
		}, processIO)
		if err != nil {
			return err
		}
	}

	processExited := make(chan struct{})

	var processStatus int
	var processErr error

	go func() {
		processStatus, processErr = process.Wait()
		close(processExited)
	}()

	select {
	case <-processExited:
		if processErr != nil {
			return processErr
		}

		if processStatus != 0 {
			return runtime.ErrResourceScriptFailed{
				Path:       path,
				Args:       args,
				ExitStatus: processStatus,

				Stderr: stderr.String(),
			}
		}

		if recoverable {
			err := container.SetProperty(runtime.ResourceResultPropertyName, stdout.String())
			if err != nil {
				return err
			}
		}

		err := json.Unmarshal(stdout.Bytes(), output)
		if err != nil {
			return fmt.Errorf("%s\n\nwhen parsing resource response:\n\n%s", err, stdout.String())
		}
		return err

	case <-ctx.Done():
		container.Stop(false)
		<-processExited
		return ctx.Err()
	}
}
