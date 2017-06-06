package exec

import (
	"archive/tar"
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
	"github.com/concourse/atc/worker"
)

const taskProcessID = "task"
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
	metadata          db.ContainerMetadata
	tags              atc.Tags
	teamID            int
	buildID           int
	planID            atc.PlanID
	delegate          TaskDelegate
	privileged        Privileged
	configSource      TaskConfigSource
	workerPool        worker.Client
	artifactsRoot     string
	resourceTypes     atc.VersionedResourceTypes
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
	metadata db.ContainerMetadata,
	tags atc.Tags,
	teamID int,
	buildID int,
	planID atc.PlanID,
	delegate TaskDelegate,
	privileged Privileged,
	configSource TaskConfigSource,
	workerPool worker.Client,
	artifactsRoot string,
	resourceTypes atc.VersionedResourceTypes,
	inputMapping map[string]string,
	outputMapping map[string]string,
	imageArtifactName string,
	clock clock.Clock,
) TaskStep {
	return TaskStep{
		logger:            logger,
		metadata:          metadata,
		tags:              tags,
		teamID:            teamID,
		buildID:           buildID,
		planID:            planID,
		delegate:          delegate,
		privileged:        privileged,
		configSource:      configSource,
		workerPool:        workerPool,
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

	containerSpec, err := step.containerSpec(config)
	if err != nil {
		return err
	}

	step.delegate.Initializing(config)

	container, err := step.workerPool.FindOrCreateContainer(
		step.logger,
		signals,
		step.delegate,
		db.ForBuild(step.buildID),
		db.NewBuildStepContainerOwner(step.buildID, step.planID),
		step.metadata,
		containerSpec,
		step.resourceTypes,
	)
	if err != nil {
		return err
	}

	exitStatusProp, err := container.Property(taskExitStatusPropertyName)
	if err == nil {
		step.logger.Info("already-exited", lager.Data{"status": exitStatusProp})

		_, err = fmt.Sscanf(exitStatusProp, "%d", &step.exitStatus)
		if err != nil {
			return err
		}

		step.registerSource(config, container)
		return nil
	}

	// for backwards compatibility with containers
	// that had their task process name set as property
	var processID string
	processID, err = container.Property(taskProcessPropertyName)
	if err != nil {
		processID = taskProcessID
	}

	step.process, err = container.Attach(processID, processIO)
	if err == nil {
		step.logger.Info("already-running")
	} else {
		step.logger.Info("spawning")

		step.delegate.Started()

		step.process, err = container.Run(garden.ProcessSpec{
			ID: taskProcessID,

			Path: config.Run.Path,
			Args: config.Run.Args,

			Dir: path.Join(step.artifactsRoot, config.Run.Dir),
			TTY: &garden.TTYSpec{},
		}, processIO)
	}
	if err != nil {
		return err
	}

	step.logger.Info("attached")

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

func (step *TaskStep) containerSpec(config atc.TaskConfig) (worker.ContainerSpec, error) {
	imageSpec := worker.ImageSpec{
		Privileged: bool(step.privileged),
	}
	if step.imageArtifactName != "" {
		source, found := step.repo.SourceFor(worker.ArtifactName(step.imageArtifactName))
		if !found {
			return worker.ContainerSpec{}, MissingTaskImageSourceError{step.imageArtifactName}
		}

		imageSpec.ImageArtifactSource = source
		imageSpec.ImageArtifactName = worker.ArtifactName(step.imageArtifactName)
	} else {
		imageSpec.ImageURL = config.RootfsURI
		imageSpec.ImageResource = config.ImageResource
	}

	containerSpec := worker.ContainerSpec{
		Platform:  config.Platform,
		Tags:      step.tags,
		TeamID:    step.teamID,
		ImageSpec: imageSpec,
		User:      config.Run.User,
		Dir:       step.artifactsRoot,
		Env:       step.envForParams(config.Params),

		Inputs:  []worker.InputSource{},
		Outputs: worker.OutputPaths{},
	}

	var missingInputs []string
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

		containerSpec.Inputs = append(containerSpec.Inputs, &taskInputSource{
			name:          worker.ArtifactName(inputName),
			config:        input,
			source:        source,
			artifactsRoot: step.artifactsRoot,
		})
	}

	if len(missingInputs) > 0 {
		return worker.ContainerSpec{}, MissingInputsError{missingInputs}
	}

	for _, output := range config.Outputs {
		path := artifactsPath(output, step.artifactsRoot)
		containerSpec.Outputs[output.Name] = path
	}

	return containerSpec, nil
}

func (step *TaskStep) registerSource(config atc.TaskConfig, container worker.Container) {
	volumeMounts := container.VolumeMounts()

	step.logger.Debug("registering-outputs", lager.Data{"config": config})

	for _, output := range config.Outputs {
		outputName := output.Name
		if destinationName, ok := step.outputMapping[output.Name]; ok {
			outputName = destinationName
		}

		outputPath := artifactsPath(output, step.artifactsRoot)

		for _, mount := range volumeMounts {
			if mount.MountPath == outputPath {
				source := newVolumeSource(step.logger, mount.Volume)
				step.repo.RegisterSource(worker.ArtifactName(outputName), source)
			}
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

func (TaskStep) envForParams(params map[string]string) []string {
	env := make([]string, 0, len(params))

	for k, v := range params {
		env = append(env, k+"="+v)
	}

	return env
}

type volumeSource struct {
	logger lager.Logger
	volume worker.Volume
}

func newVolumeSource(
	logger lager.Logger,
	volume worker.Volume,
) *volumeSource {
	return &volumeSource{
		logger: logger,
		volume: volume,
	}
}

func (src *volumeSource) StreamTo(destination worker.ArtifactDestination) error {
	out, err := src.volume.StreamOut(".")
	if err != nil {
		return err
	}

	defer out.Close()

	return destination.StreamIn(".", out)
}

func (src *volumeSource) StreamFile(filename string) (io.ReadCloser, error) {
	out, err := src.volume.StreamOut(filename)
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

func (src *volumeSource) VolumeOn(w worker.Worker) (worker.Volume, bool, error) {
	return w.LookupVolume(src.logger, src.volume.Handle())
}

type taskInputSource struct {
	name          worker.ArtifactName
	config        atc.TaskInputConfig
	source        worker.ArtifactSource
	artifactsRoot string
}

func (s *taskInputSource) Name() worker.ArtifactName     { return s.name }
func (s *taskInputSource) Source() worker.ArtifactSource { return s.source }

func (s *taskInputSource) DestinationPath() string {
	subdir := s.config.Path
	if s.config.Path == "" {
		subdir = s.config.Name
	}

	return filepath.Join(s.artifactsRoot, subdir)
}

func artifactsPath(outputConfig atc.TaskOutputConfig, artifactsRoot string) string {
	outputSrc := outputConfig.Path
	if len(outputSrc) == 0 {
		outputSrc = outputConfig.Name
	}

	return path.Join(artifactsRoot, outputSrc) + "/"
}
