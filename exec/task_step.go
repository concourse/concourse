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

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
)

const ArtifactsRoot = "/tmp/build/src"

const taskProcessPropertyName = "concourse:task-process"
const taskExitStatusPropertyName = "concourse:exit-status"

var ErrInterrupted = errors.New("interrupted")

type MissingInputsError struct {
	Inputs []string
}

func (err MissingInputsError) Error() string {
	return fmt.Sprintf("missing inputs: %s", strings.Join(err.Inputs, ", "))
}

type taskStep struct {
	SourceName SourceName

	WorkerID worker.Identifier

	Delegate TaskDelegate

	Privileged   Privileged
	ConfigSource TaskConfigSource

	WorkerClient worker.Client

	prev Step
	repo *SourceRepository

	container worker.Container
	process   garden.Process

	exitStatus int
}

func (step taskStep) Using(prev Step, repo *SourceRepository) Step {
	step.prev = prev
	step.repo = repo

	return failureReporter{
		Step:          &step,
		ReportFailure: step.Delegate.Failed,
	}
}

func (step *taskStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	var err error

	processIO := garden.ProcessIO{
		Stdout: step.Delegate.Stdout(),
		Stderr: step.Delegate.Stderr(),
	}

	step.container, err = step.WorkerClient.LookupContainer(step.WorkerID)
	if err == nil {
		// container already exists; recover session

		exitStatusProp, err := step.container.GetProperty(taskExitStatusPropertyName)
		if err == nil {
			// process already completed; recover result

			_, err = fmt.Sscanf(exitStatusProp, "%d", &step.exitStatus)
			if err != nil {
				return err
			}

			return nil
		}

		processIDProp, err := step.container.GetProperty(taskProcessPropertyName)
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

		config, err := step.ConfigSource.FetchConfig(step.repo)
		if err != nil {
			return err
		}

		step.Delegate.Initializing(config)

		step.container, err = step.WorkerClient.CreateContainer(
			step.WorkerID,
			worker.TaskContainerSpec{
				Platform:   config.Platform,
				Tags:       config.Tags,
				Image:      config.Image,
				Privileged: bool(step.Privileged),
			},
		)
		if err != nil {
			return err
		}

		err = step.ensureBuildDirExists(step.container)
		if err != nil {
			return err
		}

		err = step.collectInputs(config.Inputs)
		if err != nil {
			return err
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

		err = step.container.SetProperty(taskProcessPropertyName, pidValue)
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
		step.repo.RegisterSource(step.SourceName, step)

		step.exitStatus = status

		step.Delegate.Finished(ExitStatus(status))

		statusValue := fmt.Sprintf("%d", status)

		err := step.container.SetProperty(taskExitStatusPropertyName, statusValue)
		if err != nil {
			return err
		}

		return nil

	case err := <-waitErr:
		return err
	}
}

func (step *taskStep) Result(x interface{}) bool {
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

func (step *taskStep) Release() error {
	if step.container != nil {
		step.container.Release()
	}

	return nil
}

func (step *taskStep) StreamFile(source string) (io.ReadCloser, error) {
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

func (step *taskStep) StreamTo(destination ArtifactDestination) error {
	out, err := step.container.StreamOut(ArtifactsRoot + "/")
	if err != nil {
		return err
	}

	return destination.StreamIn(".", out)
}

func (step *taskStep) ensureBuildDirExists(container garden.Container) error {
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

func (step *taskStep) collectInputs(inputs []atc.TaskInputConfig) error {
	type inputPair struct {
		source      ArtifactSource
		destination ArtifactDestination
	}

	inputMappings := []inputPair{}

	var missingInputs []string
	for _, input := range inputs {
		source, found := step.repo.SourceFor(SourceName(input.Name))
		if !found {
			missingInputs = append(missingInputs, input.Name)
			continue
		}

		inputMappings = append(inputMappings, inputPair{
			source:      source,
			destination: newContainerDestination(step.container, input),
		})
	}

	for _, pair := range inputMappings {
		err := pair.source.StreamTo(pair.destination)
		if err != nil {
			return err
		}
	}

	if len(missingInputs) > 0 {
		return MissingInputsError{missingInputs}
	}

	return nil
}

func (taskStep) envForParams(params map[string]string) []string {
	env := make([]string, 0, len(params))

	for k, v := range params {
		env = append(env, k+"="+v)
	}

	return env
}

type containerDestination struct {
	container   garden.Container
	inputConfig atc.TaskInputConfig
}

func newContainerDestination(container garden.Container, inputConfig atc.TaskInputConfig) *containerDestination {
	return &containerDestination{
		container:   container,
		inputConfig: inputConfig,
	}
}

func (dest *containerDestination) StreamIn(dst string, src io.Reader) error {
	inputDst := dest.inputConfig.Path
	if len(inputDst) == 0 {
		inputDst = dest.inputConfig.Name
	}

	return dest.container.StreamIn(ArtifactsRoot+"/"+inputDst+"/"+dst, src)
}
