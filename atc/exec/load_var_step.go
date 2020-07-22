package exec

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"sigs.k8s.io/yaml"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/exec/artifact"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/tracing"
)

// LoadVarStep loads a value from a file and sets it as a build-local var.
type LoadVarStep struct {
	planID    atc.PlanID
	plan      atc.LoadVarPlan
	metadata  StepMetadata
	delegate  BuildStepDelegate
	client    worker.Client
	succeeded bool
}

func NewLoadVarStep(
	planID atc.PlanID,
	plan atc.LoadVarPlan,
	metadata StepMetadata,
	delegate BuildStepDelegate,
	client worker.Client,
) Step {
	return &LoadVarStep{
		planID:   planID,
		plan:     plan,
		metadata: metadata,
		delegate: delegate,
		client:   client,
	}
}

type UnspecifiedLoadVarStepFileError struct {
	File string
}

// Error returns a human-friendly error message.
func (err UnspecifiedLoadVarStepFileError) Error() string {
	return fmt.Sprintf("file '%s' does not specify where the file lives", err.File)
}

type InvalidLocalVarFile struct {
	File   string
	Format string
	Err    error
}

func (err InvalidLocalVarFile) Error() string {
	return fmt.Sprintf("failed to parse %s in format %s: %s", err.File, err.Format, err.Err.Error())
}

func (step *LoadVarStep) Run(ctx context.Context, state RunState) error {
	ctx, span := tracing.StartSpan(ctx, "load_var", tracing.Attrs{
		"team":     step.metadata.TeamName,
		"pipeline": step.metadata.PipelineName,
		"job":      step.metadata.JobName,
		"build":    step.metadata.BuildName,
		"name":     step.plan.Name,
		"file":     step.plan.File,
	})

	err := step.run(ctx, state)
	tracing.End(span, err)

	return err
}

func (step *LoadVarStep) run(ctx context.Context, state RunState) error {
	logger := lagerctx.FromContext(ctx)
	logger = logger.Session("load-var-step", lager.Data{
		"step-name": step.plan.Name,
		"job-id":    step.metadata.JobID,
	})

	step.delegate.Initializing(logger)
	stdout := step.delegate.Stdout()
	stderr := step.delegate.Stderr()

	fmt.Fprintln(stderr, "\x1b[1;33mWARNING: the load_var step is experimental and subject to change!\x1b[0m")
	fmt.Fprintln(stderr, "")
	fmt.Fprintln(stderr, "\x1b[33mfollow RFC #27 for updates: https://github.com/concourse/rfcs/pull/27\x1b[0m")
	fmt.Fprintln(stderr, "")

	step.delegate.Starting(logger)

	value, err := step.fetchVars(ctx, logger, step.plan.File, state)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "var %s fetched.\n", step.plan.Name)

	step.delegate.Variables().AddLocalVar(step.plan.Name, value, !step.plan.Reveal)
	fmt.Fprintf(stdout, "added var %s to build.\n", step.plan.Name)

	step.succeeded = true
	step.delegate.Finished(logger, step.succeeded)

	return nil
}

func (step *LoadVarStep) Succeeded() bool {
	return step.succeeded
}

func (step *LoadVarStep) fetchVars(
	ctx context.Context,
	logger lager.Logger,
	file string,
	state RunState,
) (interface{}, error) {

	segs := strings.SplitN(file, "/", 2)
	if len(segs) != 2 {
		return nil, UnspecifiedLoadVarStepFileError{file}
	}

	artifactName := segs[0]
	filePath := segs[1]

	format, err := step.fileFormat(file)
	if err != nil {
		return nil, err
	}
	logger.Debug("figure-out-format", lager.Data{"format": format})

	art, found := state.ArtifactRepository().ArtifactFor(build.ArtifactName(artifactName))
	if !found {
		return nil, UnknownArtifactSourceError{build.ArtifactName(artifactName), filePath}
	}

	stream, err := step.client.StreamFileFromArtifact(ctx, logger, art, filePath)
	if err != nil {
		if err == baggageclaim.ErrFileNotFound {
			return nil, artifact.FileNotFoundError{
				Name:     artifactName,
				FilePath: filePath,
			}
		}

		return nil, err
	}

	fileContent, err := ioutil.ReadAll(stream)
	if err != nil {
		return nil, err
	}

	var value interface{}
	switch format {
	case "json":
		value = map[string]interface{}{}
		err = json.Unmarshal(fileContent, &value)
		if err != nil {
			return nil, InvalidLocalVarFile{file, "json", err}
		}
	case "yml", "yaml":
		value = map[string]interface{}{}
		err = yaml.Unmarshal(fileContent, &value)
		if err != nil {
			return nil, InvalidLocalVarFile{file, "yaml", err}
		}
	case "trim":
		value = strings.TrimSpace(string(fileContent))
	case "raw":
		value = string(fileContent)
	default:
		return nil, fmt.Errorf("unknown format %s, should never happen, ", format)
	}

	return value, nil
}

func (step *LoadVarStep) fileFormat(file string) (string, error) {
	if step.isValidFormat(step.plan.Format) {
		return step.plan.Format, nil
	} else if step.plan.Format != "" {
		return "", fmt.Errorf("invalid format %s", step.plan.Format)
	}

	fileExt := filepath.Ext(file)
	format := strings.TrimPrefix(fileExt, ".")
	if step.isValidFormat(format) {
		return format, nil
	}

	return "trim", nil
}

func (step *LoadVarStep) isValidFormat(format string) bool {
	switch format {
	case "raw", "trim", "yml", "yaml", "json":
		return true
	}
	return false
}
