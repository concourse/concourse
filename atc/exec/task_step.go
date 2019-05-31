package exec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	boshtemplate "github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/concourse/v5/atc"
	"github.com/concourse/concourse/v5/atc/creds"
	"github.com/concourse/concourse/v5/atc/db"
	"github.com/concourse/concourse/v5/atc/exec/artifact"
	"github.com/concourse/concourse/v5/atc/worker"
)

const taskProcessID = "task"
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

type TaskImageSourceParametersError struct {
	Err error
}

func (err TaskImageSourceParametersError) Error() string {
	return fmt.Sprintf("failed to evaluate image resource parameters: %s", err.Err)
}

//go:generate counterfeiter . TaskDelegate

type TaskDelegate interface {
	BuildStepDelegate

	Initializing(lager.Logger, atc.TaskConfig)
	Starting(lager.Logger, atc.TaskConfig)
	Finished(lager.Logger, ExitStatus)
}

// TaskStep executes a TaskConfig, whose inputs will be fetched from the
// artifact.Repository and outputs will be added to the artifact.Repository.
type TaskStep struct {
	privileged    Privileged
	configSource  TaskConfigSource
	tags          atc.Tags
	inputMapping  map[string]string
	outputMapping map[string]string

	artifactsRoot     string
	imageArtifactName string

	delegate TaskDelegate

	workerPool        worker.Pool
	teamID            int
	buildID           int
	jobID             int
	stepName          string
	planID            atc.PlanID
	containerMetadata db.ContainerMetadata

	resourceTypes creds.VersionedResourceTypes

	defaultLimits atc.ContainerLimits

	succeeded bool

	strategy worker.ContainerPlacementStrategy
}

func NewTaskStep(
	privileged Privileged,
	configSource TaskConfigSource,
	tags atc.Tags,
	inputMapping map[string]string,
	outputMapping map[string]string,
	artifactsRoot string,
	imageArtifactName string,
	delegate TaskDelegate,
	workerPool worker.Pool,
	teamID int,
	buildID int,
	jobID int,
	stepName string,
	planID atc.PlanID,
	containerMetadata db.ContainerMetadata,
	resourceTypes creds.VersionedResourceTypes,
	defaultLimits atc.ContainerLimits,
	strategy worker.ContainerPlacementStrategy,
) Step {
	return &TaskStep{
		privileged:        privileged,
		configSource:      configSource,
		tags:              tags,
		inputMapping:      inputMapping,
		outputMapping:     outputMapping,
		artifactsRoot:     artifactsRoot,
		imageArtifactName: imageArtifactName,
		delegate:          delegate,
		workerPool:        workerPool,
		teamID:            teamID,
		buildID:           buildID,
		jobID:             jobID,
		stepName:          stepName,
		planID:            planID,
		containerMetadata: containerMetadata,
		resourceTypes:     resourceTypes,
		defaultLimits:     defaultLimits,
		strategy:          strategy,
	}
}

// Run will first select the worker based on the TaskConfig's platform and the
// TaskStep's tags, and prioritize it by availability of volumes for the TaskConfig's
// inputs. Inputs that did not have volumes available on the worker will be streamed
// in to the container.
//
// If any inputs are not available in the artifact.Repository, MissingInputsError
// is returned.
//
// Once all the inputs are satisfied, the task's script will be executed. If
// the task is canceled via the context, the script will be interrupted.
//
// If the script exits successfully, the outputs specified in the TaskConfig
// are registered with the artifact.Repository. If no outputs are specified, the
// task's entire working directory is registered as an ArtifactSource under the
// name of the task.
func (action *TaskStep) Run(ctx context.Context, state RunState) error {
	logger := lagerctx.FromContext(ctx)
	logger = logger.Session("task-step", lager.Data{
		"step-name": action.stepName,
		"job-id":    action.jobID,
	})

	repository := state.Artifacts()

	config, err := action.configSource.FetchConfig(logger, repository)

	for _, warning := range action.configSource.Warnings() {
		fmt.Fprintln(action.delegate.Stderr(), "[WARNING]", warning)
	}

	if err != nil {
		return err
	}
	if config.Limits.CPU == nil {
		config.Limits.CPU = action.defaultLimits.CPU
	}
	if config.Limits.Memory == nil {
		config.Limits.Memory = action.defaultLimits.Memory
	}

	action.delegate.Initializing(logger, config)

	containerSpec, err := action.containerSpec(logger, repository, config)
	if err != nil {
		return err
	}

	workerSpec, err := action.workerSpec(logger, action.resourceTypes, repository, config)
	if err != nil {
		return err
	}

	owner := db.NewBuildStepContainerOwner(action.buildID, action.planID, action.teamID)
	chosenWorker, err := action.workerPool.FindOrChooseWorkerForContainer(logger, owner, containerSpec, workerSpec, action.strategy)
	if err != nil {
		return err
	}

	container, err := chosenWorker.FindOrCreateContainer(
		ctx,
		logger,
		action.delegate,
		owner,
		action.containerMetadata,
		containerSpec,
		action.resourceTypes,
	)
	if err != nil {
		return err
	}

	exitStatusProp, err := container.Property(taskExitStatusPropertyName)
	if err == nil {
		logger.Info("already-exited", lager.Data{"status": exitStatusProp})

		status, err := strconv.Atoi(exitStatusProp)
		if err != nil {
			return err
		}

		action.succeeded = (status == 0)

		err = action.registerOutputs(logger, repository, config, container)
		if err != nil {
			return err
		}

		return nil
	}

	processIO := garden.ProcessIO{
		Stdout: action.delegate.Stdout(),
		Stderr: action.delegate.Stderr(),
	}

	process, err := container.Attach(taskProcessID, processIO)
	if err == nil {
		logger.Info("already-running")
	} else {
		logger.Info("spawning")

		action.delegate.Starting(logger, config)

		process, err = container.Run(
			garden.ProcessSpec{
				ID: taskProcessID,

				Path: config.Run.Path,
				Args: config.Run.Args,

				Dir: path.Join(action.artifactsRoot, config.Run.Dir),

				// Guardian sets the default TTY window size to width: 80, height: 24,
				// which creates ANSI control sequences that do not work with other window sizes
				TTY: &garden.TTYSpec{
					WindowSize: &garden.WindowSize{Columns: 500, Rows: 500},
				},
			},
			processIO,
		)
	}
	if err != nil {
		return err
	}

	logger.Info("attached")

	exited := make(chan struct{})
	var processStatus int
	var processErr error

	go func() {
		processStatus, processErr = process.Wait()
		close(exited)
	}()

	select {
	case <-ctx.Done():
		err = action.registerOutputs(logger, repository, config, container)
		if err != nil {
			return err
		}

		err = container.Stop(false)
		if err != nil {
			logger.Error("stopping-container", err)
		}

		<-exited

		return ctx.Err()

	case <-exited:
		if processErr != nil {
			return processErr
		}

		err = action.registerOutputs(logger, repository, config, container)
		if err != nil {
			return err
		}

		action.delegate.Finished(logger, ExitStatus(processStatus))

		err = container.SetProperty(taskExitStatusPropertyName, fmt.Sprintf("%d", processStatus))
		if err != nil {
			return err
		}

		action.succeeded = processStatus == 0

		return nil
	}
}

func (action *TaskStep) Succeeded() bool {
	return action.succeeded
}

func (action *TaskStep) imageSpec(logger lager.Logger, repository *artifact.Repository, config atc.TaskConfig) (worker.ImageSpec, error) {
	imageSpec := worker.ImageSpec{
		Privileged: bool(action.privileged),
	}

	// Determine the source of the container image
	// a reference to an artifact (get step, task output) ?
	if action.imageArtifactName != "" {
		source, found := repository.SourceFor(artifact.Name(action.imageArtifactName))
		if !found {
			return worker.ImageSpec{}, MissingTaskImageSourceError{action.imageArtifactName}
		}

		imageSpec.ImageArtifactSource = source

		//an image_resource
	} else if config.ImageResource != nil {
		imageSpec.ImageResource = &worker.ImageResource{
			Type:    config.ImageResource.Type,
			Source:  creds.NewSource(boshtemplate.StaticVariables{}, config.ImageResource.Source),
			Params:  config.ImageResource.Params,
			Version: config.ImageResource.Version,
		}
		// a rootfs_uri
	} else if config.RootfsURI != "" {
		imageSpec.ImageURL = config.RootfsURI
	}

	return imageSpec, nil
}

func (action *TaskStep) containerInputs(logger lager.Logger, repository *artifact.Repository, config atc.TaskConfig) ([]worker.InputSource, error) {
	inputs := []worker.InputSource{}

	var missingRequiredInputs []string
	for _, input := range config.Inputs {
		inputName := input.Name
		if sourceName, ok := action.inputMapping[inputName]; ok {
			inputName = sourceName
		}

		source, found := repository.SourceFor(artifact.Name(inputName))
		if !found {
			if !input.Optional {
				missingRequiredInputs = append(missingRequiredInputs, inputName)
			}
			continue
		}

		inputs = append(inputs, &taskInputSource{
			config:        input,
			source:        source,
			artifactsRoot: action.artifactsRoot,
		})
	}

	if len(missingRequiredInputs) > 0 {
		return nil, MissingInputsError{missingRequiredInputs}
	}

	for _, cacheConfig := range config.Caches {
		source := newTaskCacheSource(logger, action.teamID, action.jobID, action.stepName, cacheConfig.Path)
		inputs = append(inputs, &taskCacheInputSource{
			source:        source,
			artifactsRoot: action.artifactsRoot,
			cachePath:     cacheConfig.Path,
		})
	}

	return inputs, nil
}

func (action *TaskStep) containerSpec(logger lager.Logger, repository *artifact.Repository, config atc.TaskConfig) (worker.ContainerSpec, error) {
	imageSpec, err := action.imageSpec(logger, repository, config)
	if err != nil {
		return worker.ContainerSpec{}, err
	}

	containerSpec := worker.ContainerSpec{
		Platform:  config.Platform,
		Tags:      action.tags,
		TeamID:    action.teamID,
		ImageSpec: imageSpec,
		Limits:    worker.ContainerLimits(config.Limits),
		User:      config.Run.User,
		Dir:       action.artifactsRoot,
		Env:       action.envForParams(config.Params),

		Inputs:  []worker.InputSource{},
		Outputs: worker.OutputPaths{},
	}

	containerSpec.Inputs, err = action.containerInputs(logger, repository, config)
	if err != nil {
		return worker.ContainerSpec{}, err
	}

	for _, output := range config.Outputs {
		path := artifactsPath(output, action.artifactsRoot)
		containerSpec.Outputs[output.Name] = path
	}

	return containerSpec, nil
}

func (action *TaskStep) workerSpec(logger lager.Logger, resourceTypes creds.VersionedResourceTypes, repository *artifact.Repository, config atc.TaskConfig) (worker.WorkerSpec, error) {
	workerSpec := worker.WorkerSpec{
		Platform:      config.Platform,
		Tags:          action.tags,
		TeamID:        action.teamID,
		ResourceTypes: resourceTypes,
	}

	imageSpec, err := action.imageSpec(logger, repository, config)
	if err != nil {
		return worker.WorkerSpec{}, err
	}

	if imageSpec.ImageResource != nil {
		workerSpec.ResourceType = imageSpec.ImageResource.Type
	}

	return workerSpec, nil
}

func (action *TaskStep) registerOutputs(logger lager.Logger, repository *artifact.Repository, config atc.TaskConfig, container worker.Container) error {
	volumeMounts := container.VolumeMounts()

	logger.Debug("registering-outputs", lager.Data{"outputs": config.Outputs})

	for _, output := range config.Outputs {
		outputName := output.Name
		if destinationName, ok := action.outputMapping[output.Name]; ok {
			outputName = destinationName
		}

		outputPath := artifactsPath(output, action.artifactsRoot)

		for _, mount := range volumeMounts {
			if filepath.Clean(mount.MountPath) == filepath.Clean(outputPath) {
				source := NewTaskArtifactSource(mount.Volume)
				repository.RegisterSource(artifact.Name(outputName), source)
			}
		}
	}

	// Do not initialize caches for one-off builds
	if action.jobID != 0 {
		logger.Debug("initializing-caches", lager.Data{"caches": config.Caches})

		for _, cacheConfig := range config.Caches {
			for _, volumeMount := range volumeMounts {
				if volumeMount.MountPath == filepath.Join(action.artifactsRoot, cacheConfig.Path) {
					logger.Debug("initializing-cache", lager.Data{"path": volumeMount.MountPath})

					err := volumeMount.Volume.InitializeTaskCache(
						logger,
						action.jobID,
						action.stepName,
						cacheConfig.Path,
						bool(action.privileged))
					if err != nil {
						return err
					}

					continue
				}
			}
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

type taskArtifactSource struct {
	worker.Volume
}

func NewTaskArtifactSource(volume worker.Volume) *taskArtifactSource {
	return &taskArtifactSource{volume}
}

func (src *taskArtifactSource) StreamTo(logger lager.Logger, destination worker.ArtifactDestination) error {
	logger = logger.Session("task-artifact-streaming", lager.Data{
		"src-volume": src.Handle(),
		"src-worker": src.WorkerName(),
	})

	return streamToHelper(src, logger, destination)
}

func (src *taskArtifactSource) StreamFile(logger lager.Logger, filename string) (io.ReadCloser, error) {
	logger.Debug("streaming-file-from-volume")
	return streamFileHelper(src, logger, filename)
}

func (src *taskArtifactSource) VolumeOn(logger lager.Logger, w worker.Worker) (worker.Volume, bool, error) {
	return w.LookupVolume(logger, src.Handle())
}

type taskInputSource struct {
	config        atc.TaskInputConfig
	source        worker.ArtifactSource
	artifactsRoot string
}

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

type taskCacheInputSource struct {
	source        worker.ArtifactSource
	artifactsRoot string
	cachePath     string
}

func (s *taskCacheInputSource) Source() worker.ArtifactSource { return s.source }

func (s *taskCacheInputSource) DestinationPath() string {
	return filepath.Join(s.artifactsRoot, s.cachePath)
}

type taskCacheSource struct {
	logger   lager.Logger
	teamID   int
	jobID    int
	stepName string
	path     string
}

func newTaskCacheSource(
	logger lager.Logger,
	teamID int,
	jobID int,
	stepName string,
	path string,
) *taskCacheSource {
	return &taskCacheSource{
		logger:   logger,
		teamID:   teamID,
		jobID:    jobID,
		stepName: stepName,
		path:     path,
	}
}

func (src *taskCacheSource) StreamTo(logger lager.Logger, destination worker.ArtifactDestination) error {
	// cache will be initialized every time on a new worker
	return nil
}

func (src *taskCacheSource) StreamFile(logger lager.Logger, filename string) (io.ReadCloser, error) {
	return nil, errors.New("taskCacheSource.StreamFile not implemented")
}

func (src *taskCacheSource) VolumeOn(logger lager.Logger, w worker.Worker) (worker.Volume, bool, error) {
	return w.FindVolumeForTaskCache(src.logger, src.teamID, src.jobID, src.stepName, src.path)
}
