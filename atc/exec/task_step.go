package exec

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"io"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/vars"
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
	ImageVersionDetermined(db.UsedResourceCache) error

	Stdout() io.Writer
	Stderr() io.Writer

	Variables() vars.CredVarsTracker

	Initializing(lager.Logger, atc.TaskConfig)
	Starting(lager.Logger, atc.TaskConfig)
	Finished(lager.Logger, ExitStatus)
	Errored(lager.Logger, string)
}

// TaskStep executes a TaskConfig, whose inputs will be fetched from the
// artifact.Repository and outputs will be added to the artifact.Repository.
type TaskStep struct {
	planID            atc.PlanID
	plan              atc.TaskPlan
	defaultLimits     atc.ContainerLimits
	metadata          StepMetadata
	containerMetadata db.ContainerMetadata
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
// task's entire working directory is registered as an StreamableArtifactSource under the
// name of the task.
func (step *TaskStep) Run(ctx context.Context, state RunState) error {
	logger := lagerctx.FromContext(ctx)
	logger = logger.Session("task-step", lager.Data{
		"step-name": step.plan.Name,
		"job-id":    step.metadata.JobID,
	})

	variables := step.delegate.Variables()

	resourceTypes, err := creds.NewVersionedResourceTypes(variables, step.plan.VersionedResourceTypes).Evaluate()
	if err != nil {
		return err
	}

	var taskConfigSource TaskConfigSource
	var taskVars []vars.Variables

	if step.plan.ConfigPath != "" {
		// external task - construct a source which reads it from file
		taskConfigSource = FileConfigSource{ConfigPath: step.plan.ConfigPath, Client: step.workerClient}

		// for interpolation - use 'vars' from the pipeline, and then fill remaining with cred variables
		taskVars = []vars.Variables{vars.StaticVariables(step.plan.Vars), variables}
	} else {
		// embedded task - first we take it
		taskConfigSource = StaticConfigSource{Config: step.plan.Config}

		// for interpolation - use just cred variables
		taskVars = []vars.Variables{variables}
	}

	// override params
	taskConfigSource = &OverrideParamsConfigSource{ConfigSource: taskConfigSource, Params: step.plan.Params}

	// interpolate template vars
	taskConfigSource = InterpolateTemplateConfigSource{ConfigSource: taskConfigSource, Vars: taskVars}

	// validate
	taskConfigSource = ValidatingConfigSource{ConfigSource: taskConfigSource}

	repository := state.ArtifactRepository()

	config, err := taskConfigSource.FetchConfig(ctx, logger, repository)

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

	step.delegate.Initializing(logger, config)

	workerSpec, err := step.workerSpec(logger, resourceTypes, repository, config)
	if err != nil {
		return err
	}

	containerSpec, err := step.containerSpec(logger, repository, config, step.containerMetadata)
	if err != nil {
		return err
	}

	processSpec := runtime.ProcessSpec{
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

	step.delegate.Starting(logger, config)

	result := step.workerClient.RunTaskStep(
		ctx,
		logger,
		owner,
		containerSpec,
		workerSpec,
		step.strategy,
		step.containerMetadata,
		imageSpec,
		processSpec,
		step.lockFactory,
	)

	err = result.Err
	if err != nil {
		if err == context.Canceled || err == context.DeadlineExceeded {
			step.registerOutputs(logger, repository, config, result.VolumeMounts, step.containerMetadata)
		}
		return err
	}

	step.succeeded = (result.Status == 0)
	step.delegate.Finished(logger, ExitStatus(result.Status))

	step.registerOutputs(logger, repository, config, result.VolumeMounts, step.containerMetadata)

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

func (step *TaskStep) imageSpec(logger lager.Logger, repository *build.Repository, config atc.TaskConfig) (worker.ImageSpec, error) {
	imageSpec := worker.ImageSpec{
		Privileged: bool(step.plan.Privileged),
	}

	// Determine the source of the container image
	// a reference to an artifact (get step, task output) ?
	if step.plan.ImageArtifactName != "" {
		art, found := repository.ArtifactFor(build.ArtifactName(step.plan.ImageArtifactName))
		if !found {
			return worker.ImageSpec{}, MissingTaskImageSourceError{step.plan.ImageArtifactName}
		}

		imageSpec.ImageArtifact = art

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

func (step *TaskStep) containerInputs(logger lager.Logger, repository *build.Repository, config atc.TaskConfig, metadata db.ContainerMetadata) (map[string]runtime.Artifact, error) {
	inputs := map[string]runtime.Artifact{}

	var missingRequiredInputs []string

	for _, input := range config.Inputs {
		inputName := input.Name
		if sourceName, ok := step.plan.InputMapping[inputName]; ok {
			inputName = sourceName
		}

		art, found := repository.ArtifactFor(build.ArtifactName(inputName))
		if !found {
			if !input.Optional {
				missingRequiredInputs = append(missingRequiredInputs, inputName)
			}
			continue
		}
		ti := taskInput{
			config:        input,
			artifact:      art,
			artifactsRoot: metadata.WorkingDirectory,
		}

		inputs[ti.Path()] = ti.Artifact()
	}

	if len(missingRequiredInputs) > 0 {
		return nil, MissingInputsError{missingRequiredInputs}
	}

	for _, cacheConfig := range config.Caches {
		cacheArt := &runtime.CacheArtifact{
			TeamID:   step.metadata.TeamID,
			JobID:    step.metadata.JobID,
			StepName: step.plan.Name,
			Path:     cacheConfig.Path,
		}
		ti := taskCacheInput{
			artifact:      cacheArt,
			artifactsRoot: metadata.WorkingDirectory,
			cachePath:     cacheConfig.Path,
		}
		inputs[ti.Path()] = ti.Artifact()
	}

	return inputs, nil
}

func (step *TaskStep) containerSpec(logger lager.Logger, repository *build.Repository, config atc.TaskConfig, metadata db.ContainerMetadata) (worker.ContainerSpec, error) {
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
		Env:       config.Params.Env(),
		Type:      metadata.Type,

		Outputs:      worker.OutputPaths{},
	}

	containerSpec.ArtifactByPath, err = step.containerInputs(logger, repository, config, metadata)
	if err != nil {
		return worker.ContainerSpec{}, err
	}

	for _, output := range config.Outputs {
		path := artifactsPath(output, metadata.WorkingDirectory)
		containerSpec.Outputs[output.Name] = path
	}

	return containerSpec, nil
}

func (step *TaskStep) workerSpec(logger lager.Logger, resourceTypes atc.VersionedResourceTypes, repository *build.Repository, config atc.TaskConfig) (worker.WorkerSpec, error) {
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

func (step *TaskStep) registerOutputs(logger lager.Logger, repository *build.Repository, config atc.TaskConfig, volumeMounts []worker.VolumeMount, metadata db.ContainerMetadata) {
	logger.Debug("registering-outputs", lager.Data{"outputs": config.Outputs})

	for _, output := range config.Outputs {
		outputName := output.Name
		if destinationName, ok := step.plan.OutputMapping[output.Name]; ok {
			outputName = destinationName
		}

		outputPath := artifactsPath(output, metadata.WorkingDirectory)

		for _, mount := range volumeMounts {
			if filepath.Clean(mount.MountPath) == filepath.Clean(outputPath) {
				art := &runtime.TaskArtifact{
					VolumeHandle: mount.Volume.Handle(),
				}
				repository.RegisterArtifact(build.ArtifactName(outputName), art)
			}
		}
	}
}

func (step *TaskStep) registerCaches(logger lager.Logger, repository *build.Repository, config atc.TaskConfig, volumeMounts []worker.VolumeMount, metadata db.ContainerMetadata) error {
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

type taskInput struct {
	config        atc.TaskInputConfig
	artifact      runtime.Artifact
	artifactsRoot string
}

func (s taskInput) Artifact() runtime.Artifact { return s.artifact }

func (s taskInput) Path() string {
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

type taskCacheInput struct {
	artifact      runtime.Artifact
	artifactsRoot string
	cachePath     string
}

func (s taskCacheInput) Artifact() runtime.Artifact { return s.artifact }

func (s taskCacheInput) Path() string {
	return filepath.Join(s.artifactsRoot, s.cachePath)
}
