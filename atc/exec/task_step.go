package exec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/vars"
	"go.opentelemetry.io/otel/trace"
)

const taskProcessID = "task"

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

//counterfeiter:generate . TaskDelegateFactory
type TaskDelegateFactory interface {
	TaskDelegate(state RunState) TaskDelegate
}

//counterfeiter:generate . TaskDelegate
type TaskDelegate interface {
	StartSpan(context.Context, string, tracing.Attrs) (context.Context, trace.Span)

	FetchImage(context.Context, atc.ImageResource, atc.ResourceTypes, bool, atc.Tags) (runtime.ImageSpec, error)

	Stdout() io.Writer
	Stderr() io.Writer

	SetTaskConfig(config atc.TaskConfig)

	Initializing(lager.Logger)
	Starting(lager.Logger)
	Finished(lager.Logger, ExitStatus)
	Errored(lager.Logger, string)

	WaitingForWorker(lager.Logger)
	SelectedWorker(lager.Logger, string)
}

// TaskStep executes a TaskConfig, whose inputs will be fetched from the
// artifact.Repository and outputs will be added to the artifact.Repository.
type TaskStep struct {
	planID            atc.PlanID
	plan              atc.TaskPlan
	defaultLimits     atc.ContainerLimits
	metadata          StepMetadata
	containerMetadata db.ContainerMetadata
	strategy          worker.PlacementStrategy
	workerPool        Pool
	streamer          Streamer
	delegateFactory   TaskDelegateFactory
}

func NewTaskStep(
	planID atc.PlanID,
	plan atc.TaskPlan,
	defaultLimits atc.ContainerLimits,
	metadata StepMetadata,
	containerMetadata db.ContainerMetadata,
	strategy worker.PlacementStrategy,
	workerPool Pool,
	streamer Streamer,
	delegateFactory TaskDelegateFactory,
) Step {
	return &TaskStep{
		planID:            planID,
		plan:              plan,
		defaultLimits:     defaultLimits,
		metadata:          metadata,
		containerMetadata: containerMetadata,
		strategy:          strategy,
		workerPool:        workerPool,
		streamer:          streamer,
		delegateFactory:   delegateFactory,
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
func (step *TaskStep) Run(ctx context.Context, state RunState) (bool, error) {
	delegate := step.delegateFactory.TaskDelegate(state)
	ctx, span := delegate.StartSpan(ctx, "task", tracing.Attrs{
		"name": step.plan.Name,
	})

	ok, err := step.run(ctx, state, delegate)
	tracing.End(span, err)

	return ok, err
}

func (step *TaskStep) run(ctx context.Context, state RunState, delegate TaskDelegate) (bool, error) {
	logger := lagerctx.FromContext(ctx)
	logger = logger.Session("task-step", lager.Data{
		"step-name": step.plan.Name,
		"job-id":    step.metadata.JobID,
	})

	var taskConfigSource TaskConfigSource
	var taskVars []vars.Variables

	if step.plan.ConfigPath != "" {
		// external task - construct a source which reads it from file, and apply base resource type defaults.
		taskConfigSource = FileConfigSource{ConfigPath: step.plan.ConfigPath, Streamer: step.streamer}

		// for interpolation - use 'vars' from the pipeline, and then fill remaining with cred variables.
		// this 2-phase strategy allows to interpolate 'vars' by cred variables.
		if len(step.plan.Vars) > 0 {
			taskConfigSource = InterpolateTemplateConfigSource{
				ConfigSource:  taskConfigSource,
				Vars:          []vars.Variables{vars.StaticVariables(step.plan.Vars)},
				ExpectAllKeys: false,
			}
		}
		taskVars = []vars.Variables{state}
	} else {
		// embedded task - first we take it
		taskConfigSource = StaticConfigSource{Config: step.plan.Config}

		// for interpolation - use just cred variables
		taskVars = []vars.Variables{state}
	}

	// apply resource type defaults
	taskConfigSource = BaseResourceTypeDefaultsApplySource{
		ConfigSource:  taskConfigSource,
		ResourceTypes: step.plan.ResourceTypes,
	}

	// override limits
	taskConfigSource = &OverrideContainerLimitsSource{ConfigSource: taskConfigSource, Limits: step.plan.Limits}

	// override params
	taskConfigSource = &OverrideParamsConfigSource{ConfigSource: taskConfigSource, Params: step.plan.Params}

	// interpolate template vars
	taskConfigSource = InterpolateTemplateConfigSource{
		ConfigSource:  taskConfigSource,
		Vars:          taskVars,
		ExpectAllKeys: true,
	}

	// validate
	taskConfigSource = ValidatingConfigSource{ConfigSource: taskConfigSource}

	repository := state.ArtifactRepository()

	config, err := taskConfigSource.FetchConfig(ctx, logger, repository)

	delegate.SetTaskConfig(config)

	for _, warning := range taskConfigSource.Warnings() {
		fmt.Fprintln(delegate.Stderr(), "[WARNING]", warning)
	}

	if err != nil {
		return false, err
	}

	if config.Limits == nil {
		config.Limits = &atc.ContainerLimits{}
	}
	if config.Limits.CPU == nil {
		config.Limits.CPU = step.defaultLimits.CPU
	}
	if config.Limits.Memory == nil {
		config.Limits.Memory = step.defaultLimits.Memory
	}

	delegate.Initializing(logger)

	imageSpec, err := step.imageSpec(ctx, logger, state, delegate, config)
	if err != nil {
		return false, err
	}

	containerSpec, err := step.containerSpec(logger, state, imageSpec, config, step.containerMetadata)
	if err != nil {
		return false, err
	}
	tracing.Inject(ctx, &containerSpec)

	owner := db.NewBuildStepContainerOwner(step.metadata.BuildID, step.planID, step.metadata.TeamID)

	worker, err := step.workerPool.FindOrSelectWorker(
		ctx,
		owner,
		containerSpec,
		step.workerSpec(config),
		step.strategy,
		delegate,
	)
	if err != nil {
		return false, err
	}

	defer func() {
		step.workerPool.ReleaseWorker(
			logger,
			containerSpec,
			worker,
			step.strategy,
		)
	}()

	if step.plan.Timeout != "" {
		timeout, err := time.ParseDuration(step.plan.Timeout)
		if err != nil {
			return false, fmt.Errorf("parse timeout: %w", err)
		}

		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	ctx = lagerctx.NewContext(ctx, logger)

	delegate.SelectedWorker(logger, worker.Name())

	container, volumeMounts, err := worker.FindOrCreateContainer(ctx, owner, step.containerMetadata, containerSpec)
	if err != nil {
		return false, err
	}

	delegate.Starting(logger)
	process, err := attachOrRun(
		ctx,
		container,
		runtime.ProcessSpec{
			ID:   taskProcessID,
			Path: config.Run.Path,
			Args: config.Run.Args,
			Dir:  resolvePath(step.containerMetadata.WorkingDirectory, config.Run.Dir),
			User: config.Run.User,
			// Guardian sets the default TTY window size to width: 80, height: 24,
			// which creates ANSI control sequences that do not work with other window sizes
			TTY: &runtime.TTYSpec{
				WindowSize: runtime.WindowSize{
					Columns: 500,
					Rows:    500,
				},
			},
		},
		runtime.ProcessIO{
			Stdout: delegate.Stdout(),
			Stderr: delegate.Stderr(),
		},
	)
	if err != nil {
		return false, err
	}

	result, runErr := process.Wait(ctx)

	step.registerOutputs(logger, repository, config, volumeMounts, step.containerMetadata)

	// Do not initialize caches for one-off builds
	if step.metadata.JobID != 0 {
		if err := step.registerCaches(logger, repository, config, volumeMounts, step.containerMetadata); err != nil {
			return false, err
		}
	}

	if runErr != nil {
		if errors.Is(runErr, context.DeadlineExceeded) {
			delegate.Errored(logger, TimeoutLogMessage)
			return false, nil
		}

		return false, runErr
	}

	delegate.Finished(logger, ExitStatus(result.ExitStatus))
	return result.ExitStatus == 0, nil
}

func attachOrRun(ctx context.Context, container runtime.Container, spec runtime.ProcessSpec, io runtime.ProcessIO) (runtime.Process, error) {
	process, err := container.Attach(ctx, spec.ID, io)
	if err == nil {
		return process, nil
	}
	return container.Run(ctx, spec, io)
}

func (step *TaskStep) imageSpec(ctx context.Context, logger lager.Logger, state RunState, delegate TaskDelegate, config atc.TaskConfig) (runtime.ImageSpec, error) {
	imageSpec := runtime.ImageSpec{
		Privileged: bool(step.plan.Privileged),
	}

	// Determine the source of the container image
	// a reference to an artifact (get step, task output) ?
	if step.plan.ImageArtifactName != "" {
		artifact, found := state.ArtifactRepository().ArtifactFor(build.ArtifactName(step.plan.ImageArtifactName))
		if !found {
			return runtime.ImageSpec{}, MissingTaskImageSourceError{step.plan.ImageArtifactName}
		}
		imageSpec.ImageArtifact = artifact

		//an image_resource
	} else if config.ImageResource != nil {
		imageSpec, err := delegate.FetchImage(
			ctx,
			*config.ImageResource,
			step.plan.ResourceTypes,
			step.plan.Privileged,
			step.plan.Tags,
		)
		return imageSpec, err

		// a rootfs_uri
	} else if config.RootfsURI != "" {
		imageSpec.ImageURL = config.RootfsURI
	}

	return imageSpec, nil
}

func (step *TaskStep) containerInputs(logger lager.Logger, repository *build.Repository, config atc.TaskConfig, metadata db.ContainerMetadata) ([]runtime.Input, error) {
	var inputs []runtime.Input

	var missingRequiredInputs []string

	for _, input := range config.Inputs {
		inputName := input.Name
		if sourceName, ok := step.plan.InputMapping[inputName]; ok {
			inputName = sourceName
		}

		artifact, found := repository.ArtifactFor(build.ArtifactName(inputName))
		if !found {
			if !input.Optional {
				missingRequiredInputs = append(missingRequiredInputs, inputName)
			}
			continue
		}
		inputs = append(inputs, runtime.Input{
			Artifact:        artifact,
			DestinationPath: artifactPath(metadata.WorkingDirectory, input.Name, input.Path),
		})
	}

	if len(missingRequiredInputs) > 0 {
		return nil, MissingInputsError{missingRequiredInputs}
	}

	return inputs, nil
}

func (step *TaskStep) containerSpec(logger lager.Logger, state RunState, imageSpec runtime.ImageSpec, config atc.TaskConfig, metadata db.ContainerMetadata) (runtime.ContainerSpec, error) {
	containerSpec := runtime.ContainerSpec{
		TeamID:   step.metadata.TeamID,
		TeamName: step.metadata.TeamName,
		JobID:    step.metadata.JobID,
		StepName: step.plan.Name,

		ImageSpec: imageSpec,
		Env:       config.Params.Env(),
		Type:      metadata.Type,

		Dir: metadata.WorkingDirectory,
	}

	var err error
	containerSpec.Inputs, err = step.containerInputs(logger, state.ArtifactRepository(), config, metadata)
	if err != nil {
		return runtime.ContainerSpec{}, err
	}

	containerSpec.Caches = make([]string, len(config.Caches))
	for i, cache := range config.Caches {
		containerSpec.Caches[i] = cache.Path
	}

	containerSpec.Outputs = make(runtime.OutputPaths, len(config.Outputs))
	for _, output := range config.Outputs {
		containerSpec.Outputs[output.Name] = ensureTrailingSlash(artifactPath(metadata.WorkingDirectory, output.Name, output.Path))
	}

	if config.Limits != nil {
		containerSpec.Limits.CPU = (*uint64)(config.Limits.CPU)
		containerSpec.Limits.Memory = (*uint64)(config.Limits.Memory)
	}

	return containerSpec, nil
}

func (step *TaskStep) workerSpec(config atc.TaskConfig) worker.Spec {
	return worker.Spec{
		Platform: config.Platform,
		Tags:     step.plan.Tags,
		TeamID:   step.metadata.TeamID,
	}
}

func (step *TaskStep) registerOutputs(logger lager.Logger, repository *build.Repository, config atc.TaskConfig, volumeMounts []runtime.VolumeMount, metadata db.ContainerMetadata) {
	logger.Debug("registering-outputs", lager.Data{"outputs": config.Outputs})

	for _, output := range config.Outputs {
		outputName := output.Name
		if destinationName, ok := step.plan.OutputMapping[output.Name]; ok {
			outputName = destinationName
		}

		outputPath := artifactPath(metadata.WorkingDirectory, output.Name, output.Path)

		for _, mount := range volumeMounts {
			if filepath.Clean(mount.MountPath) == filepath.Clean(outputPath) {
				repository.RegisterArtifact(build.ArtifactName(outputName), mount.Volume)
			}
		}
	}
}

func (step *TaskStep) registerCaches(logger lager.Logger, repository *build.Repository, config atc.TaskConfig, volumeMounts []runtime.VolumeMount, metadata db.ContainerMetadata) error {
	for _, cacheConfig := range config.Caches {
		for _, volumeMount := range volumeMounts {
			mountPath := resolvePath(metadata.WorkingDirectory, cacheConfig.Path)
			if filepath.Clean(volumeMount.MountPath) == mountPath {
				logger.Debug("initializing-cache", lager.Data{
					"cache": cacheConfig.Path,
				})
				err := volumeMount.Volume.InitializeTaskCache(
					logger,
					step.metadata.JobID,
					step.plan.Name,
					cacheConfig.Path,
					step.plan.Privileged,
				)
				if err != nil {
					return err
				}

				break
			}
		}
	}

	return nil
}

func artifactPath(workingDir string, name string, path string) string {
	subdir := path
	if path == "" {
		subdir = name
	}

	return resolvePath(workingDir, subdir)
}

func resolvePath(workingDir string, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(workingDir, path)
}

func ensureTrailingSlash(path string) string {
	if strings.HasSuffix(path, "/") {
		return path
	}
	return path + "/"
}
