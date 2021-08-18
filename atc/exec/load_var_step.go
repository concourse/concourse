package exec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"sigs.k8s.io/yaml"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/worker/baggageclaim"
)

// LoadVarStep loads a value from a file and sets it as a build-local var.
type LoadVarStep struct {
	planID          atc.PlanID
	plan            atc.LoadVarPlan
	metadata        StepMetadata
	delegateFactory BuildStepDelegateFactory
	streamer        Streamer
}

func NewLoadVarStep(
	planID atc.PlanID,
	plan atc.LoadVarPlan,
	metadata StepMetadata,
	delegateFactory BuildStepDelegateFactory,
	streamer Streamer,
) Step {
	return &LoadVarStep{
		planID:          planID,
		plan:            plan,
		metadata:        metadata,
		delegateFactory: delegateFactory,
		streamer:        streamer,
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

func (step *LoadVarStep) Run(ctx context.Context, state RunState) (bool, error) {
	delegate := step.delegateFactory.BuildStepDelegate(state)
	ctx, span := delegate.StartSpan(ctx, "load_var", tracing.Attrs{
		"name": step.plan.Name,
	})

	ok, err := step.run(ctx, state, delegate)
	tracing.End(span, err)

	return ok, err
}

func (step *LoadVarStep) run(ctx context.Context, state RunState, delegate BuildStepDelegate) (bool, error) {
	logger := lagerctx.FromContext(ctx)
	logger = logger.Session("load-var-step", lager.Data{
		"step-name": step.plan.Name,
		"job-id":    step.metadata.JobID,
	})

	delegate.Initializing(logger)
	stdout := delegate.Stdout()

	delegate.Starting(logger)

	value, err := step.fetchVars(ctx, logger, step.plan.File, state)
	if err != nil {
		return false, err
	}
	fmt.Fprintf(stdout, "var %s fetched.\n", step.plan.Name)

	state.AddLocalVar(step.plan.Name, value, !step.plan.Reveal)
	fmt.Fprintf(stdout, "added var %s to build.\n", step.plan.Name)

	delegate.Finished(logger, true)

	return true, nil
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

	stream, err := step.streamer.StreamFile(lagerctx.NewContext(ctx, logger), art, filePath)
	if err != nil {
		if err == baggageclaim.ErrFileNotFound {
			return nil, FileNotFoundError{
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
		decoder := json.NewDecoder(bytes.NewReader(fileContent))
		decoder.UseNumber()
		err = decoder.Decode(&value)
		if err != nil {
			return nil, InvalidLocalVarFile{file, "json", err}
		}
		if decoder.More() {
			return nil, InvalidLocalVarFile{file, "json", errors.New("invalid json: characters found after top-level value")}
		}
	case "yml", "yaml":
		err = yaml.Unmarshal(fileContent, &value, useJSONNumber)
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

func useJSONNumber(decoder *json.Decoder) *json.Decoder {
	decoder.UseNumber()
	return decoder
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

// FileNotFoundError is returned when the specified file path does not
// exist within its artifact source.
type FileNotFoundError struct {
	Name     string
	FilePath string
}

// Error returns a human-friendly error message.
func (err FileNotFoundError) Error() string {
	return fmt.Sprintf("file '%s' not found within artifact '%s'", err.FilePath, err.Name)
}
