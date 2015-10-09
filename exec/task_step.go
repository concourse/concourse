package exec

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
	"github.com/concourse/baggageclaim"
	"github.com/pivotal-golang/lager"
)

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
	logger        lager.Logger
	sourceName    SourceName
	containerID   worker.Identifier
	tags          atc.Tags
	delegate      TaskDelegate
	privileged    Privileged
	configSource  TaskConfigSource
	workerPool    worker.Client
	artifactsRoot string

	repo *SourceRepository

	container worker.Container
	process   garden.Process

	exitStatus int
}

func newTaskStep(
	logger lager.Logger,
	sourceName SourceName,
	containerID worker.Identifier,
	tags atc.Tags,
	delegate TaskDelegate,
	privileged Privileged,
	configSource TaskConfigSource,
	workerPool worker.Client,
	artifactsRoot string,
) taskStep {
	return taskStep{
		logger:        logger,
		sourceName:    sourceName,
		containerID:   containerID,
		tags:          tags,
		delegate:      delegate,
		privileged:    privileged,
		configSource:  configSource,
		workerPool:    workerPool,
		artifactsRoot: artifactsRoot,
	}
}

func (step taskStep) Using(prev Step, repo *SourceRepository) Step {
	step.repo = repo

	return failureReporter{
		Step:          &step,
		ReportFailure: step.delegate.Failed,
	}
}

func (step *taskStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	var err error
	var found bool

	processIO := garden.ProcessIO{
		Stdout: step.delegate.Stdout(),
		Stderr: step.delegate.Stderr(),
	}

	step.container, found, err = step.workerPool.FindContainerForIdentifier(
		step.logger.Session("found-container"),
		step.containerID,
	)
	if err == nil && found {
		exitStatusProp, err := step.container.Property(taskExitStatusPropertyName)
		if err == nil {
			// process already completed; recover result

			_, err = fmt.Sscanf(exitStatusProp, "%d", &step.exitStatus)
			if err != nil {
				return err
			}

			return nil
		}

		processID, err := step.container.Property(taskProcessPropertyName)
		if err != nil {
			// rogue container? perhaps did not shut down cleanly.
			return err
		}

		// process still running; re-attach
		step.process, err = step.container.Attach(processID, processIO)
		if err != nil {
			return err
		}
	} else {
		// container does not exist; new session

		config, err := step.configSource.FetchConfig(step.repo)
		if err != nil {
			return err
		}

		tags := step.mergeTags(step.tags, config.Tags)

		step.delegate.Initializing(config)

		workerSpec := worker.WorkerSpec{
			Platform: config.Platform,
			Tags:     tags,
		}

		compatibleWorkers, err := step.workerPool.AllSatisfying(workerSpec)
		if err != nil {
			return err
		}

		// find the worker with the most volumes
		inputMounts := []worker.VolumeMount{}
		inputsToStream := []inputPair{}
		var chosenWorker worker.Worker
		for _, w := range compatibleWorkers {
			mounts, toStream, err := step.inputsOn(config.Inputs, w)
			if err != nil {
				return err
			}
			if len(mounts) >= len(inputMounts) {
				for _, mount := range inputMounts {
					mount.Volume.Release(0)
				}
				inputMounts = mounts
				inputsToStream = toStream
				chosenWorker = w
			} else {
				for _, mount := range mounts {
					mount.Volume.Release(0)
				}
			}
		}

		step.container, err = chosenWorker.CreateContainer(
			step.logger.Session("created-container"),
			step.containerID,
			worker.TaskContainerSpec{
				Platform:   config.Platform,
				Tags:       tags,
				Image:      config.Image,
				Privileged: bool(step.privileged),
				Inputs:     inputMounts,
			},
		)
		if err != nil {
			return err
		}

		for _, mount := range inputMounts {
			// stop heartbeating ourselves now that container has picked up the
			// volumes
			mount.Volume.Release(0)
		}

		err = step.ensureBuildDirExists(step.container)
		if err != nil {
			return err
		}

		err = step.streamInputs(inputsToStream)
		if err != nil {
			return err
		}

		step.delegate.Started()

		step.process, err = step.container.Run(garden.ProcessSpec{
			Path: config.Run.Path,
			Args: config.Run.Args,
			Env:  step.envForParams(config.Params),

			Dir:  step.artifactsRoot,
			User: "root",
			TTY:  &garden.TTYSpec{},
		}, processIO)
		if err != nil {
			return err
		}

		err = step.container.SetProperty(taskProcessPropertyName, step.process.ID())
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
		step.repo.RegisterSource(step.sourceName, step)

		step.exitStatus = status

		step.delegate.Finished(ExitStatus(status))

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

func (step *taskStep) Release() {
	if step.container == nil {
		return
	}

	if step.exitStatus == 0 {
		step.container.Release(successfulStepTTL)
	} else {
		step.container.Release(failedStepTTL)
	}
}

func (step *taskStep) StreamFile(source string) (io.ReadCloser, error) {
	out, err := step.container.StreamOut(garden.StreamOutSpec{
		Path: path.Join(step.artifactsRoot, source),
	})

	if err != nil {
		return nil, err
	}

	tarReader := tar.NewReader(out)

	_, err = tarReader.Next()
	if err != nil {
		return nil, FileNotFoundError{Path: source}
	}

	return fileReadCloser{
		Reader: tarReader,
		Closer: out,
	}, nil
}

func (step *taskStep) StreamTo(destination ArtifactDestination) error {
	out, err := step.container.StreamOut(garden.StreamOutSpec{
		Path: step.artifactsRoot + "/",
	})
	if err != nil {
		return err
	}

	return destination.StreamIn(".", out)
}

func (step *taskStep) VolumeOn(worker worker.Worker) (baggageclaim.Volume, bool, error) {
	return nil, false, nil
}

type inputPair struct {
	input  atc.TaskInputConfig
	source ArtifactSource
}

func (step *taskStep) inputsOn(inputs []atc.TaskInputConfig, chosenWorker worker.Worker) ([]worker.VolumeMount, []inputPair, error) {
	var mounts []worker.VolumeMount

	var inputPairs []inputPair

	var missingInputs []string

	for _, input := range inputs {
		source, found := step.repo.SourceFor(SourceName(input.Name))
		if !found {
			missingInputs = append(missingInputs, input.Name)
			continue
		}

		volume, existsOnWorker, err := source.VolumeOn(chosenWorker)
		if err != nil {
			return nil, nil, err
		}

		if existsOnWorker {
			mounts = append(mounts, worker.VolumeMount{
				Volume:    volume,
				MountPath: step.inputDestination(input),
			})
		} else {
			inputPairs = append(inputPairs, inputPair{
				input:  input,
				source: source,
			})
		}
	}

	if len(missingInputs) > 0 {
		return nil, nil, MissingInputsError{missingInputs}
	}

	return mounts, inputPairs, nil
}

func (step *taskStep) inputDestination(config atc.TaskInputConfig) string {
	subdir := config.Path
	if config.Path == "" {
		subdir = config.Name
	}

	return filepath.Join(step.artifactsRoot, subdir)
}

func (step *taskStep) ensureBuildDirExists(container garden.Container) error {
	emptyTar := new(bytes.Buffer)

	err := tar.NewWriter(emptyTar).Close()
	if err != nil {
		return err
	}

	err = container.StreamIn(garden.StreamInSpec{
		Path:      step.artifactsRoot,
		TarStream: emptyTar,
	})
	if err != nil {
		return err
	}

	return nil
}

func (step *taskStep) streamInputs(inputPairs []inputPair) error {
	for _, pair := range inputPairs {
		destination := newContainerDestination(
			step.artifactsRoot,
			step.container,
			pair.input,
		)

		err := pair.source.StreamTo(destination)
		if err != nil {
			return err
		}
	}

	return nil
}

func (taskStep) mergeTags(tagsOne []string, tagsTwo []string) []string {
	var ret []string

	uniq := map[string]struct{}{}

	for _, tag := range tagsOne {
		uniq[tag] = struct{}{}
	}

	for _, tag := range tagsTwo {
		uniq[tag] = struct{}{}
	}

	for tag, _ := range uniq {
		ret = append(ret, tag)
	}

	return ret
}

func (taskStep) envForParams(params map[string]string) []string {
	env := make([]string, 0, len(params))

	for k, v := range params {
		env = append(env, k+"="+v)
	}

	return env
}

type containerDestination struct {
	container     garden.Container
	inputConfig   atc.TaskInputConfig
	artifactsRoot string
}

func newContainerDestination(artifactsRoot string, container garden.Container, inputConfig atc.TaskInputConfig) *containerDestination {
	return &containerDestination{
		container:     container,
		inputConfig:   inputConfig,
		artifactsRoot: artifactsRoot,
	}
}

func (dest *containerDestination) StreamIn(dst string, src io.Reader) error {
	inputDst := dest.inputConfig.Path
	if len(inputDst) == 0 {
		inputDst = dest.inputConfig.Name
	}

	return dest.container.StreamIn(garden.StreamInSpec{
		Path:      dest.artifactsRoot + "/" + inputDst + "/" + dst,
		TarStream: src,
	})
}
