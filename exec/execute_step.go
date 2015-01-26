package exec

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
)

const ArtifactsRoot = "/tmp/build/src"

const executeProcessPropertyName = "execute-process"
const executeExitStatusPropertyName = "exit-status"

var ErrInterrupted = errors.New("interrupted")

type MissingInputsError struct {
	Inputs []string
}

func (err MissingInputsError) Error() string {
	return fmt.Sprintf("missing inputs: %s", strings.Join(err.Inputs, ", "))
}

type executeStep struct {
	SessionID SessionID

	Delegate ExecuteDelegate

	Privileged   Privileged
	ConfigSource BuildConfigSource

	WorkerClient worker.Client

	artifactSource ArtifactSource

	container worker.Container
	process   garden.Process

	exitStatus int
}

func (step executeStep) Using(source ArtifactSource) ArtifactSource {
	step.artifactSource = source

	return failureReporter{
		ArtifactSource: &step,
		ReportFailure:  step.Delegate.Failed,
	}
}

func (step *executeStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	var err error

	processIO := garden.ProcessIO{
		Stdout: step.Delegate.Stdout(),
		Stderr: step.Delegate.Stderr(),
	}

	step.container, err = step.WorkerClient.Lookup(string(step.SessionID))
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

		step.Delegate.Initializing(config)

		step.container, err = step.WorkerClient.Create(garden.ContainerSpec{
			Handle:     string(step.SessionID),
			RootFSPath: config.Image,
			Privileged: bool(step.Privileged),
		})
		if err != nil {
			return err
		}

		err = step.ensureBuildDirExists(step.container)
		if err != nil {
			return err
		}

		dest := newContainerDestination(step.container, config.Inputs)

		err = step.artifactSource.StreamTo(dest)
		if err != nil {
			return err
		}

		missing := dest.MissingInputs()

		if len(missing) > 0 {
			return MissingInputsError{missing}
		}

		step.Delegate.Started()

		step.process, err = step.container.Run(garden.ProcessSpec{
			Path: config.Run.Path,
			Args: config.Run.Args,
			Env:  step.envForParams(config.Params),

			Dir: ArtifactsRoot,

			Privileged: bool(step.Privileged),

			TTY: &garden.TTYSpec{},
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

	defer step.container.Release()

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

		step.Delegate.Finished(ExitStatus(status))

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
	return step.container.Destroy()
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
	out, err := step.container.StreamOut(ArtifactsRoot + "/")
	if err != nil {
		return err
	}

	return destination.StreamIn(".", out)
}

func (step *executeStep) ensureBuildDirExists(container garden.Container) error {
	emptyTar := new(bytes.Buffer)

	err := tar.NewWriter(emptyTar).Close()
	if err != nil {
		return err
	}

	err = container.StreamIn(ArtifactsRoot, emptyTar)
	if err != nil {
		return err
	}

	return nil
}

func (executeStep) envForParams(params map[string]string) []string {
	env := make([]string, 0, len(params))

	for k, v := range params {
		env = append(env, k+"="+v)
	}

	return env
}

type containerDestination struct {
	container    garden.Container
	inputConfigs []atc.BuildInputConfig

	missingInputs map[string]struct{}

	lock sync.Mutex
}

func newContainerDestination(container garden.Container, inputs []atc.BuildInputConfig) *containerDestination {
	missingInputs := map[string]struct{}{}

	for _, i := range inputs {
		missingInputs[i.Name] = struct{}{}
	}

	return &containerDestination{
		container:    container,
		inputConfigs: inputs,

		missingInputs: missingInputs,
	}
}

func (dest *containerDestination) StreamIn(dst string, src io.Reader) error {
	destSegments := strings.Split(dst, "/")

	if len(destSegments) > 0 {
		dest.lock.Lock()
		delete(dest.missingInputs, destSegments[0])
		dest.lock.Unlock()

		for _, config := range dest.inputConfigs {
			if config.Name == destSegments[0] && config.Path != "" {
				destSegments[0] = config.Path
				break
			}
		}
	}

	return dest.container.StreamIn(path.Join(ArtifactsRoot, strings.Join(destSegments, "/")), src)
}

func (dest *containerDestination) MissingInputs() []string {
	dest.lock.Lock()
	defer dest.lock.Unlock()

	missing := make([]string, 0, len(dest.missingInputs))

	for i, _ := range dest.missingInputs {
		missing = append(missing, i)
	}

	return missing
}
