package exec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/cloudfoundry/bosh-cli/director/template"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/exec/artifact"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
)

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
	planID            atc.PlanID
	plan              atc.TaskPlan
	defaultLimits     atc.ContainerLimits
	metadata          StepMetadata
	containerMetadata db.ContainerMetadata
	secrets           creds.Secrets
	strategy          worker.ContainerPlacementStrategy
	workerClient      worker.Client
	delegate          TaskDelegate
	lockFactory       lock.LockFactory
	succeeded         bool
}

func NewTaskStep(
	planID atc.PlanID,
	plan atc.TaskPlan,
	defaultLimits atc.ContainerLimits,
	metadata StepMetadata,
	containerMetadata db.ContainerMetadata,
	secrets creds.Secrets,
	strategy worker.ContainerPlacementStrategy,
	workerClient worker.Client,
	delegate TaskDelegate,
	lockFactory lock.LockFactory,
) Step {
	return &TaskStep{
		planID:            planID,
		plan:              plan,
		defaultLimits:     defaultLimits,
		metadata:          metadata,
		containerMetadata: containerMetadata,
		secrets:           secrets,
		strategy:          strategy,
		workerClient:      workerClient,
		delegate:          delegate,
		lockFactory:       lockFactory,
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
func (step *TaskStep) Run(ctx context.Context, state RunState) error {
	logger := lagerctx.FromContext(ctx)
	logger = logger.Session("task-step", lager.Data{
		"step-name": step.plan.Name,
		"job-id":    step.metadata.JobID,
	})

	variables := creds.NewVariables(step.secrets, step.metadata.TeamName, step.metadata.PipelineName)

	resourceTypes, err := creds.NewVersionedResourceTypes(variables, step.plan.VersionedResourceTypes).Evaluate()
	if err != nil {
		return err
	}

	var taskConfigSource TaskConfigSource
	var taskVars []template.Variables

	if step.plan.ConfigPath != "" {
		// external task - construct a source which reads it from file
		taskConfigSource = FileConfigSource{ConfigPath: step.plan.ConfigPath}

		// for interpolation - use 'vars' from the pipeline, and then fill remaining with cred variables
		taskVars = []template.Variables{template.StaticVariables(step.plan.Vars), variables}
	} else {
		// embedded task - first we take it
		taskConfigSource = StaticConfigSource{Config: step.plan.Config}

		// for interpolation - use just cred variables
		taskVars = []template.Variables{variables}
	}

	// override params
	taskConfigSource = &OverrideParamsConfigSource{ConfigSource: taskConfigSource, Params: step.plan.Params}

	// interpolate template vars
	taskConfigSource = InterpolateTemplateConfigSource{ConfigSource: taskConfigSource, Vars: taskVars}

	// validate
	taskConfigSource = ValidatingConfigSource{ConfigSource: taskConfigSource}

	repository := state.Artifacts()

	config, err := taskConfigSource.FetchConfig(logger, repository)

	for _, warning := range taskConfigSource.Warnings() {
		fmt.Fprintln(step.delegate.Stderr(), "[WARNING]", warning)
	}

	if err != nil {
		return err
	}

	if config.Limits.CPU == nil {
		config.Limits.CPU = step.defaultLimits.CPU
	}
	if config.Limits.Memory == nil {
		config.Limits.Memory = step.defaultLimits.Memory
	}

	events := make(chan runtime.Event, 1)
	go func(logger lager.Logger, config atc.TaskConfig, events chan runtime.Event, delegate TaskDelegate) {
		for ev := range events {
			switch ev.EventType {
			case runtime.InitializingEvent:
				step.delegate.Initializing(logger, config)

			case runtime.StartingEvent:
				step.delegate.Starting(logger, config)

			case runtime.FinishedEvent:
				step.delegate.Finished(logger, ExitStatus(ev.ExitStatus))
			}
		}
	}(logger, config, events, step.delegate)

	step.delegate.Initializing(logger, config)

	workerSpec, err := step.workerSpec(logger, resourceTypes, repository, config)
	if err != nil {
		return err
	}

	containerSpec, err := step.containerSpec(logger, repository, config, step.containerMetadata)
	if err != nil {
		return err
	}

	processSpec := worker.TaskProcessSpec{
		Path:         config.Run.Path,
		Args:         config.Run.Args,
		Dir:          config.Run.Dir,
		StdoutWriter: step.delegate.Stdout(),
		StderrWriter: step.delegate.Stderr(),
	}

	imageSpec := worker.ImageFetcherSpec{
		ResourceTypes: resourceTypes,
		Delegate:      step.delegate,
	}
	owner := db.NewBuildStepContainerOwner(step.metadata.BuildID, step.planID, step.metadata.TeamID)

	result := step.workerClient.RunTaskStep(
		ctx,
		logger,
		step.lockFactory,
		owner,
		containerSpec,
		workerSpec,
		step.strategy,
		step.containerMetadata,
		imageSpec,
		processSpec,
		events,
	)

	close(events)

	err = result.Err
	if err != nil {
		if err == context.Canceled || err == context.DeadlineExceeded {
			registerErr := step.registerOutputs(logger, repository, config, result.VolumeMounts, step.containerMetadata)
			if registerErr != nil {
				return registerErr
			}
		}
		return err
	}

	step.succeeded = (result.Status == 0)
	step.delegate.Finished(logger, ExitStatus(result.Status))

	err = step.registerOutputs(logger, repository, config, result.VolumeMounts, step.containerMetadata)
	if err != nil {
		return err
	}

	// Do not initialize caches for one-off builds
	if step.metadata.JobID != 0 {
		err = step.registerCaches(logger, repository, config, result.VolumeMounts, step.containerMetadata)
		if err != nil {
			return err
		}
	}

	return nil

}

func (step *TaskStep) Succeeded() bool {
	return step.succeeded
}

func (step *TaskStep) imageSpec(logger lager.Logger, repository *artifact.Repository, config atc.TaskConfig) (worker.ImageSpec, error) {
	imageSpec := worker.ImageSpec{
		Privileged: bool(step.plan.Privileged),
	}

	// Determine the source of the container image
	// a reference to an artifact (get step, task output) ?
	if step.plan.ImageArtifactName != "" {
		source, found := repository.SourceFor(artifact.Name(step.plan.ImageArtifactName))
		if !found {
			return worker.ImageSpec{}, MissingTaskImageSourceError{step.plan.ImageArtifactName}
		}

		imageSpec.ImageArtifactSource = source

		//an image_resource
	} else if config.ImageResource != nil {
		imageSpec.ImageResource = &worker.ImageResource{
			Type:    config.ImageResource.Type,
			Source:  config.ImageResource.Source,
			Params:  config.ImageResource.Params,
			Version: config.ImageResource.Version,
		}
		// a rootfs_uri
	} else if config.RootfsURI != "" {
		imageSpec.ImageURL = config.RootfsURI
	}

	return imageSpec, nil
}

func (step *TaskStep) containerInputs(logger lager.Logger, repository *artifact.Repository, config atc.TaskConfig, metadata db.ContainerMetadata) ([]worker.InputSource, error) {
	inputs := []worker.InputSource{}

	var missingRequiredInputs []string
	for _, input := range config.Inputs {
		inputName := input.Name
		if sourceName, ok := step.plan.InputMapping[inputName]; ok {
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
			artifactsRoot: metadata.WorkingDirectory,
		})
	}

	if len(missingRequiredInputs) > 0 {
		return nil, MissingInputsError{missingRequiredInputs}
	}

	for _, cacheConfig := range config.Caches {
		source := newTaskCacheSource(logger, step.metadata.TeamID, step.metadata.JobID, step.plan.Name, cacheConfig.Path)
		inputs = append(inputs, &taskCacheInputSource{
			source:        source,
			artifactsRoot: metadata.WorkingDirectory,
			cachePath:     cacheConfig.Path,
		})
	}

	return inputs, nil
}

func (step *TaskStep) containerSpec(logger lager.Logger, repository *artifact.Repository, config atc.TaskConfig, metadata db.ContainerMetadata) (worker.ContainerSpec, error) {
	imageSpec, err := step.imageSpec(logger, repository, config)
	if err != nil {
		return worker.ContainerSpec{}, err
	}

	containerSpec := worker.ContainerSpec{
		Platform:  config.Platform,
		Tags:      step.plan.Tags,
		TeamID:    step.metadata.TeamID,
		ImageSpec: imageSpec,
		Limits:    worker.ContainerLimits(config.Limits),
		User:      config.Run.User,
		Dir:       metadata.WorkingDirectory,
		Env:       step.envForParams(config.Params),

		Inputs:  []worker.InputSource{},
		Outputs: worker.OutputPaths{},
	}

	containerSpec.Inputs, err = step.containerInputs(logger, repository, config, metadata)
	if err != nil {
		return worker.ContainerSpec{}, err
	}

	for _, output := range config.Outputs {
		path := artifactsPath(output, metadata.WorkingDirectory)
		containerSpec.Outputs[output.Name] = path
	}

	return containerSpec, nil
}

func (step *TaskStep) workerSpec(logger lager.Logger, resourceTypes atc.VersionedResourceTypes, repository *artifact.Repository, config atc.TaskConfig) (worker.WorkerSpec, error) {
	workerSpec := worker.WorkerSpec{
		Platform:      config.Platform,
		Tags:          step.plan.Tags,
		TeamID:        step.metadata.TeamID,
		ResourceTypes: resourceTypes,
	}

	imageSpec, err := step.imageSpec(logger, repository, config)
	if err != nil {
		return worker.WorkerSpec{}, err
	}

	if imageSpec.ImageResource != nil {
		workerSpec.ResourceType = imageSpec.ImageResource.Type
	}

	return workerSpec, nil
}

func (step *TaskStep) registerOutputs(logger lager.Logger, repository *artifact.Repository, config atc.TaskConfig, volumeMounts []worker.VolumeMount, metadata db.ContainerMetadata) error {
	logger.Debug("registering-outputs", lager.Data{"outputs": config.Outputs})

	for _, output := range config.Outputs {
		outputName := output.Name
		if destinationName, ok := step.plan.OutputMapping[output.Name]; ok {
			outputName = destinationName
		}

		outputPath := artifactsPath(output, metadata.WorkingDirectory)

		for _, mount := range volumeMounts {
			if filepath.Clean(mount.MountPath) == filepath.Clean(outputPath) {
				source := NewTaskArtifactSource(mount.Volume)
				repository.RegisterSource(artifact.Name(outputName), source)
			}
		}
	}

	return nil
}

func (step *TaskStep) registerCaches(logger lager.Logger, repository *artifact.Repository, config atc.TaskConfig, volumeMounts []worker.VolumeMount, metadata db.ContainerMetadata) error {
	logger.Debug("initializing-caches", lager.Data{"caches": config.Caches})

	for _, cacheConfig := range config.Caches {
		for _, volumeMount := range volumeMounts {
			if volumeMount.MountPath == filepath.Join(metadata.WorkingDirectory, cacheConfig.Path) {
				logger.Debug("initializing-cache", lager.Data{"path": volumeMount.MountPath})

				err := volumeMount.Volume.InitializeTaskCache(
					logger,
					step.metadata.JobID,
					step.plan.Name,
					cacheConfig.Path,
					bool(step.plan.Privileged))
				if err != nil {
					return err
				}

				continue
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
