package exec

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

const taskProcessPropertyName = "concourse:task-process"
const taskExitStatusPropertyName = "concourse:exit-status"

// MissingInputsError is returned when any of the task's required inputs are
// missing.
type MissingInputsError struct {
	Inputs []string
}

// Error prints a human-friendly message listing the inputs that were missing.
func (err MissingInputsError) Error() string {
	return fmt.Sprintf("missing inputs: %s", strings.Join(err.Inputs, ", "))
}

type MissingTaskImageSourceError struct {
	SourceName string
}

func (err MissingTaskImageSourceError) Error() string {
	return fmt.Sprintf(`missing image artifact source: %s
make sure there's a corresponding 'get' step, or a task that produces it as an output`, err.SourceName)
}

// TaskStep executes a TaskConfig, whose inputs will be fetched from the
// worker.ArtifactRepository and outputs will be added to the worker.ArtifactRepository.
type TaskStep struct {
	logger            lager.Logger
	containerID       worker.Identifier
	metadata          worker.Metadata
	tags              atc.Tags
	teamID            int
	delegate          TaskDelegate
	privileged        Privileged
	configSource      TaskConfigSource
	workerPool        worker.Client
	resourceFactory   resource.ResourceFactory
	artifactsRoot     string
	resourceTypes     atc.ResourceTypes
	inputMapping      map[string]string
	outputMapping     map[string]string
	imageArtifactName string
	clock             clock.Clock
	repo              *worker.ArtifactRepository

	process garden.Process

	exitStatus int
}

func newTaskStep(
	logger lager.Logger,
	containerID worker.Identifier,
	metadata worker.Metadata,
	tags atc.Tags,
	teamID int,
	delegate TaskDelegate,
	privileged Privileged,
	configSource TaskConfigSource,
	workerPool worker.Client,
	resourceFactory resource.ResourceFactory,
	artifactsRoot string,
	resourceTypes atc.ResourceTypes,
	inputMapping map[string]string,
	outputMapping map[string]string,
	imageArtifactName string,
	clock clock.Clock,
) TaskStep {
	return TaskStep{
		logger:            logger,
		containerID:       containerID,
		metadata:          metadata,
		tags:              tags,
		teamID:            teamID,
		delegate:          delegate,
		privileged:        privileged,
		configSource:      configSource,
		workerPool:        workerPool,
		resourceFactory:   resourceFactory,
		artifactsRoot:     artifactsRoot,
		resourceTypes:     resourceTypes,
		inputMapping:      inputMapping,
		outputMapping:     outputMapping,
		imageArtifactName: imageArtifactName,
		clock:             clock,
	}
}

// Using finishes construction of the TaskStep and returns a *TaskStep. If the
// *TaskStep errors, its error is reported to the delegate.
func (step TaskStep) Using(prev Step, repo *worker.ArtifactRepository) Step {
	step.repo = repo

	return errorReporter{
		Step:          &step,
		ReportFailure: step.delegate.Failed,
	}
}

// Run will first load the TaskConfig. A worker will be selected based on the
// TaskConfig's platform, the TaskStep's tags, and prioritized by availability
// of volumes for the TaskConfig's inputs. Inputs that did not have volumes
// available on the worker will be streamed in to the container.
//
// If any inputs are not available in the worker.ArtifactRepository, MissingInputsError
// is returned.
//
// Once all the inputs are satisfies, the task's script will be executed, and
// the RunStep indicates that it's ready, and any signals will be forwarded to
// the script.
//
// If the script exits successfully, the outputs specified in the TaskConfig
// are registered with the worker.ArtifactRepository. If no outputs are specified, the
// task's entire working directory is registered as an ArtifactSource under the
// name of the task.
func (step *TaskStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	var err error
	var found bool

	processIO := garden.ProcessIO{
		Stdout: step.delegate.Stdout(),
		Stderr: step.delegate.Stderr(),
	}

	deprecationConfigSource := DeprecationConfigSource{
		Delegate: step.configSource,
		Stderr:   step.delegate.Stderr(),
	}

	config, err := deprecationConfigSource.FetchConfig(step.repo)
	if err != nil {
		return err
	}

	step.metadata.EnvironmentVariables = step.envForParams(config.Params)

	runContainerID := step.containerID
	runContainerID.Stage = db.ContainerStageRun

	container, found, err := step.workerPool.FindContainerForIdentifier(
		step.logger.Session("found-container"),
		runContainerID,
	)

	if err == nil && found {
		exitStatusProp, err := container.Property(taskExitStatusPropertyName)
		if err == nil {
			step.logger.Info("already-exited", lager.Data{"status": exitStatusProp})

			// process already completed; recover result

			_, err = fmt.Sscanf(exitStatusProp, "%d", &step.exitStatus)
			if err != nil {
				return err
			}

			step.registerSource(config, container)
			return nil
		}

		processID, err := container.Property(taskProcessPropertyName)
		if err != nil {
			// rogue container? perhaps did not shut down cleanly.
			return err
		}

		step.logger.Info("already-running", lager.Data{"process-id": processID})

		// process still running; re-attach
		step.process, err = container.Attach(processID, processIO)
		if err != nil {
			return err
		}

		step.logger.Info("attached")
	} else {
		// container does not exist; new session

		step.delegate.Initializing(config)

		workerSpec := worker.WorkerSpec{
			Platform: config.Platform,
			Tags:     step.tags,
			TeamID:   step.teamID,
		}

		if config.ImageResource != nil {
			workerSpec.ResourceType = config.ImageResource.Type
		}

		runResource, inputsToStream, err := step.createContainer(config, signals)
		if err != nil {
			return err
		}
		container = runResource.Container()

		err = step.ensureBuildDirExists(container)
		if err != nil {
			return err
		}

		err = step.streamInputs(inputsToStream, container)
		if err != nil {
			return err
		}

		err = step.setupOutputs(config.Outputs, container)
		if err != nil {
			return err
		}

		step.delegate.Started()

		step.process, err = container.Run(garden.ProcessSpec{
			Path: config.Run.Path,
			Args: config.Run.Args,
			Env:  step.envForParams(config.Params),

			Dir: path.Join(step.artifactsRoot, config.Run.Dir),
			TTY: &garden.TTYSpec{},
		}, processIO)
		if err != nil {
			return err
		}

		err = container.SetProperty(taskProcessPropertyName, step.process.ID())
		if err != nil {
			return err
		}
	}

	close(ready)

	exited := make(chan struct{})
	var processStatus int
	var processErr error

	go func() {
		processStatus, processErr = step.process.Wait()
		close(exited)
	}()

	select {
	case <-signals:
		step.registerSource(config, container)

		err = container.Stop(false)
		if err != nil {
			step.logger.Error("stopping-container", err)
		}

		<-exited

		return ErrInterrupted

	case <-exited:
		if processErr != nil {
			return processErr
		}

		step.registerSource(config, container)

		step.exitStatus = processStatus

		err := container.SetProperty(taskExitStatusPropertyName, fmt.Sprintf("%d", processStatus))
		if err != nil {
			return err
		}

		step.delegate.Finished(ExitStatus(processStatus))

		return nil
	}
}

func (step *TaskStep) createContainer(config atc.TaskConfig, signals <-chan os.Signal) (resource.Resource, []resource.InputSource, error) {
	outputPaths := map[string]string{}
	for _, output := range config.Outputs {
		path := artifactsPath(output, step.artifactsRoot)
		outputPaths[output.Name] = path
	}

	imageSpec := worker.ImageSpec{
		Privileged: bool(step.privileged),
	}
	if step.imageArtifactName != "" {
		source, found := step.repo.SourceFor(worker.ArtifactName(step.imageArtifactName))
		if !found {
			return nil, nil, MissingTaskImageSourceError{step.imageArtifactName}
		}

		imageSpec.ImageArtifactSource = source
		imageSpec.ImageArtifactName = worker.ArtifactName(step.imageArtifactName)
	} else {
		imageSpec.ImageURL = config.Image
		imageSpec.ImageResource = config.ImageResource
	}

	runContainerID := step.containerID
	runContainerID.Stage = db.ContainerStageRun

	var missingInputs []string
	inputSources := []resource.InputSource{}
	for _, input := range config.Inputs {
		inputName := input.Name
		if sourceName, ok := step.inputMapping[inputName]; ok {
			inputName = sourceName
		}

		source, found := step.repo.SourceFor(worker.ArtifactName(inputName))
		if !found {
			missingInputs = append(missingInputs, inputName)
			continue
		}

		inputSources = append(inputSources, &taskInputSource{
			name:          worker.ArtifactName(inputName),
			config:        input,
			source:        source,
			artifactsRoot: step.artifactsRoot,
		})
	}

	if len(missingInputs) > 0 {
		return nil, nil, MissingInputsError{missingInputs}
	}

	containerSpec := worker.ContainerSpec{
		Platform:  config.Platform,
		Tags:      step.tags,
		TeamID:    step.teamID,
		ImageSpec: imageSpec,
		User:      config.Run.User,
	}

	resource, missingInputSources, err := step.resourceFactory.NewBuildResource(
		step.logger,
		runContainerID,
		step.metadata,
		containerSpec,
		step.resourceTypes,
		step.delegate,
		inputSources,
		outputPaths,
	)
	if err != nil {
		return nil, nil, err
	}

	return resource, missingInputSources, nil
}

func (step *TaskStep) registerSource(config atc.TaskConfig, container worker.Container) {
	volumeMounts := container.VolumeMounts()

	step.logger.Debug("registering-outputs", lager.Data{"config": config})

	for _, output := range config.Outputs {
		outputName := output.Name
		if destinationName, ok := step.outputMapping[output.Name]; ok {
			outputName = destinationName
		}

		if len(volumeMounts) > 0 {
			outputPath := artifactsPath(output, step.artifactsRoot)

			for _, mount := range volumeMounts {
				if mount.MountPath == outputPath {

					source := newContainerSource(step.artifactsRoot, container, output, step.logger, mount.Volume.Handle())
					step.repo.RegisterSource(worker.ArtifactName(outputName), source)
				}
			}
		} else {
			step.logger.Debug("container-has-volume-mounts-NONE")
			source := newContainerSource(step.artifactsRoot, container, output, step.logger, "")
			step.repo.RegisterSource(worker.ArtifactName(outputName), source)
		}
	}
}

// Result indicates Success as true if the script's exit status was 0.
//
// It also indicates ExitStatus as the exit status of the script.
//
// All other types are ignored.
func (step *TaskStep) Result(x interface{}) bool {
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

func (step *TaskStep) ensureBuildDirExists(container worker.Container) error {
	return createContainerDir(container, step.artifactsRoot)
}

func (step *TaskStep) streamInputs(inputSources []resource.InputSource, container worker.Container) error {
	for _, inputSource := range inputSources {
		destination := newContainerDestination(
			inputSource,
			container,
		)

		err := inputSource.Source().StreamTo(destination)
		if err != nil {
			return err
		}
	}

	return nil
}

func (step *TaskStep) setupOutputs(outputs []atc.TaskOutputConfig, container worker.Container) error {
	for _, output := range outputs {
		source := newContainerSource(step.artifactsRoot, container, output, step.logger, "")

		err := source.initialize()
		if err != nil {
			return err
		}
	}

	return nil
}

func (TaskStep) envForParams(params map[string]string) []string {
	env := make([]string, 0, len(params))

	for k, v := range params {
		env = append(env, k+"="+v)
	}

	return env
}

type containerDestination struct {
	container   garden.Container
	inputSource resource.InputSource
}

func newContainerDestination(inputSource resource.InputSource, container worker.Container) *containerDestination {
	return &containerDestination{
		container:   container,
		inputSource: inputSource,
	}
}

func (dest *containerDestination) StreamIn(dst string, src io.Reader) error {
	return dest.container.StreamIn(garden.StreamInSpec{
		Path:      filepath.Join(dest.inputSource.MountPath(), dst),
		TarStream: src,
	})
}

type containerSource struct {
	container     garden.Container
	outputConfig  atc.TaskOutputConfig
	artifactsRoot string
	volumeHandle  string
	logger        lager.Logger
}

func newContainerSource(
	artifactsRoot string,
	container garden.Container,
	outputConfig atc.TaskOutputConfig,
	logger lager.Logger,
	volumeHandle string,
) *containerSource {
	return &containerSource{
		container:     container,
		outputConfig:  outputConfig,
		artifactsRoot: artifactsRoot,
		volumeHandle:  volumeHandle,
		logger:        logger,
	}
}

func (src *containerSource) StreamTo(destination worker.ArtifactDestination) error {
	out, err := src.container.StreamOut(garden.StreamOutSpec{
		Path: artifactsPath(src.outputConfig, src.artifactsRoot),
	})
	if err != nil {
		return err
	}

	defer out.Close()

	return destination.StreamIn(".", out)
}

func (src *containerSource) StreamFile(filename string) (io.ReadCloser, error) {
	out, err := src.container.StreamOut(garden.StreamOutSpec{
		Path: path.Join(artifactsPath(src.outputConfig, src.artifactsRoot), filename),
	})
	if err != nil {
		return nil, err
	}

	tarReader := tar.NewReader(out)

	_, err = tarReader.Next()
	if err != nil {
		return nil, FileNotFoundError{Path: filename}
	}

	return fileReadCloser{
		Reader: tarReader,
		Closer: out,
	}, nil
}

func (src *containerSource) VolumeOn(w worker.Worker) (worker.Volume, bool, error) {
	return w.LookupVolume(src.logger, src.volumeHandle)
}

func artifactsPath(outputConfig atc.TaskOutputConfig, artifactsRoot string) string {
	outputSrc := outputConfig.Path
	if len(outputSrc) == 0 {
		outputSrc = outputConfig.Name
	}

	return path.Join(artifactsRoot, outputSrc) + "/"
}

func (src *containerSource) initialize() error {
	return createContainerDir(src.container, artifactsPath(src.outputConfig, src.artifactsRoot))
}

func createContainerDir(container garden.Container, dir string) error {
	emptyTar := new(bytes.Buffer)

	err := tar.NewWriter(emptyTar).Close()
	if err != nil {
		return err
	}

	err = container.StreamIn(garden.StreamInSpec{
		Path:      dir,
		TarStream: emptyTar,
	})
	if err != nil {
		return err
	}

	return nil
}

type taskInputSource struct {
	name          worker.ArtifactName
	config        atc.TaskInputConfig
	source        worker.ArtifactSource
	artifactsRoot string
}

func (s *taskInputSource) Name() worker.ArtifactName     { return s.name }
func (s *taskInputSource) Source() worker.ArtifactSource { return s.source }
func (s *taskInputSource) MountPath() string {
	subdir := s.config.Path
	if s.config.Path == "" {
		subdir = s.config.Name
	}

	return filepath.Join(s.artifactsRoot, subdir)
}
