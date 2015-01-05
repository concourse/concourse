package exec

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	garden "github.com/cloudfoundry-incubator/garden/api"
	"github.com/concourse/atc"
)

const ArtifactsRoot = "/tmp/build/src"

const executeProcessPropertyName = "execute-process"
const executeExitStatusPropertyName = "exit-status"

var ErrInterrupted = errors.New("interrupted")

type executeStep struct {
	SessionID SessionID

	IOConfig IOConfig

	ConfigSource BuildConfigSource

	GardenClient garden.Client

	artifactSource ArtifactSource

	container garden.Container
	process   garden.Process

	exitStatus int
}

func (step executeStep) Using(source ArtifactSource) ArtifactSource {
	step.artifactSource = source
	return &step
}

func (step *executeStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	var err error

	processIO := garden.ProcessIO{
		Stdout: step.IOConfig.Stdout,
		Stderr: step.IOConfig.Stderr,
	}

	step.container, err = step.GardenClient.Lookup(string(step.SessionID))
	if err == nil {
		// container already exists; recover session

		exitStatusProp, err := step.container.GetProperty(executeExitStatusPropertyName)
		if err == nil {
			// process already completed; recover result

			_, err = fmt.Sscanf(exitStatusProp, "%d", &step.exitStatus)
			if err != nil {
				return err
			}

			return nil
		}

		processIDProp, err := step.container.GetProperty(executeProcessPropertyName)
		if err != nil {
			// rogue container? perhaps did not shut down cleanly.
			return err
		}

		// process still running; re-attach
		var processID uint32
		_, err = fmt.Sscanf(processIDProp, "%d", &processID)
		if err != nil {
			return err
		}

		step.process, err = step.container.Attach(processID, processIO)
		if err != nil {
			return err
		}
	} else {
		// container does not exist; new session

		config, err := step.ConfigSource.FetchConfig(step.artifactSource)
		if err != nil {
			return err
		}

		step.container, err = step.GardenClient.Create(garden.ContainerSpec{
			Handle:     string(step.SessionID),
			RootFSPath: config.Image,
		})
		if err != nil {
			return err
		}

		err = step.artifactSource.StreamTo(containerDestination{
			Container:    step.container,
			InputConfigs: config.Inputs,
		})
		if err != nil {
			return err
		}

		step.process, err = step.container.Run(garden.ProcessSpec{
			Path: config.Run.Path,
			Args: config.Run.Args,
			Env:  step.envForParams(config.Params),

			Dir: ArtifactsRoot,
		}, processIO)
		if err != nil {
			return err
		}

		pidValue := fmt.Sprintf("%d", step.process.ID())

		err = step.container.SetProperty(executeProcessPropertyName, pidValue)
		if err != nil {
			return err
		}
	}

	close(ready)

	waitExitStatus := make(chan int, 1)
	waitErr := make(chan error, 1)
	go func() {
		status, err := step.process.Wait()
		if err != nil {
			waitErr <- err
		} else {
			waitExitStatus <- status
		}
	}()

	select {
	case <-signals:
		step.container.Stop(false)
		return ErrInterrupted

	case status := <-waitExitStatus:
		step.exitStatus = status

		statusValue := fmt.Sprintf("%d", status)

		err := step.container.SetProperty(executeExitStatusPropertyName, statusValue)
		if err != nil {
			return err
		}

		return nil

	case err := <-waitErr:
		return err
	}
}

func (step *executeStep) Result(x interface{}) bool {
	switch v := x.(type) {
	case *Success:
		*v = step.exitStatus == 0
		return true

	case *ExitStatus:
		*v = ExitStatus(step.exitStatus)
		return true

	default:
		return false
	}
}

func (step *executeStep) Release() error {
	return step.GardenClient.Destroy(step.container.Handle())
}

func (step *executeStep) StreamFile(source string) (io.ReadCloser, error) {
	out, err := step.container.StreamOut(path.Join(ArtifactsRoot, source))
	if err != nil {
		return nil, err
	}

	tarReader := tar.NewReader(out)

	_, err = tarReader.Next()
	if err != nil {
		return nil, ErrFileNotFound
	}

	return fileReadCloser{
		Reader: tarReader,
		Closer: out,
	}, nil
}

func (step *executeStep) StreamTo(destination ArtifactDestination) error {
	out, err := step.container.StreamOut(ArtifactsRoot)
	if err != nil {
		return err
	}

	return destination.StreamIn(".", out)
}

func (executeStep) envForParams(params map[string]string) []string {
	env := make([]string, 0, len(params))

	for k, v := range params {
		env = append(env, k+"="+v)
	}

	return env
}

type containerDestination struct {
	Container garden.Container

	InputConfigs []atc.BuildInputConfig
}

func (dest containerDestination) StreamIn(dst string, src io.Reader) error {
	destSegments := strings.Split(dst, "/")

	if len(destSegments) > 0 {
		for _, config := range dest.InputConfigs {
			if config.Name == destSegments[0] {
				destSegments[0] = config.Path
				break
			}
		}
	}

	return dest.Container.StreamIn(path.Join(ArtifactsRoot, strings.Join(destSegments, "/")), src)
}
