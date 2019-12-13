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
	"github.com/concourse/concourse/vars"
)

// SetPipelineStep sets a pipeline to current team. This step takes pipeline
// configure file and var files from some resource in the pipeline, like git.
type VarStep struct {
	planID    atc.PlanID
	plan      atc.VarPlan
	metadata  StepMetadata
	delegate  BuildStepDelegate
	client    worker.Client
	succeeded bool
}

func NewVarStep(
	planID atc.PlanID,
	plan atc.VarPlan,
	metadata StepMetadata,
	delegate BuildStepDelegate,
	client worker.Client,
) Step {
	return &VarStep{
		planID:   planID,
		plan:     plan,
		metadata: metadata,
		delegate: delegate,
		client:   client,
	}
}

func (step *VarStep) Run(ctx context.Context, state RunState) error {
	logger := lagerctx.FromContext(ctx)
	logger = logger.Session("put-step", lager.Data{
		"step-name": step.plan.Name,
		"job-id":    step.metadata.JobID,
	})

	step.delegate.Initializing(logger)
	stdout := step.delegate.Stdout()
	stderr := step.delegate.Stderr()

	step.delegate.Starting(logger)

	varFromFile, err := step.fetchVars(ctx, logger, step.plan.Name, step.plan.File, state)
	if err != nil {
		logger.Error("failed to fetch var file", err)
		fmt.Fprint(stderr, "failed to fetch var file %s: %s.\n", step.plan.File, err.Error())
		step.delegate.Finished(logger, false)
		return nil
	}
	fmt.Fprintf(stdout, "var %s fetched.\n", step.plan.Name)

	step.delegate.Variables().AddVar(varFromFile)
	fmt.Fprintf(stdout, "added var %s to build.\n", step.plan.Name)

	step.succeeded = true
	step.delegate.Finished(logger, step.succeeded)

	return nil
}

func (step *VarStep) Succeeded() bool {
	return step.succeeded
}

func (step *VarStep) fetchVars(ctx context.Context, logger lager.Logger, varName string, file string, state RunState) (vars.Variables, error) {
	segs := strings.SplitN(file, "/", 2)
	if len(segs) != 2 {
		return nil, UnspecifiedArtifactSourceError{file}
	}

	artifactName := segs[0]
	filePath := segs[1]

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

	byteConfig, err := ioutil.ReadAll(stream)
	if err != nil {
		return nil, err
	}

	varFromFile := vars.StaticVariables{}
	switch filepath.Ext(filePath) {
	case ".json":
		value := map[string]interface{}{}
		err = json.Unmarshal(byteConfig, &value)
		if err != nil {
			return nil, err
		}
		varFromFile[varName] = value
	case ".yml", ".yaml":
		value := map[string]interface{}{}
		err = yaml.Unmarshal(byteConfig, &value)
		if err != nil {
			return nil, err
		}
		varFromFile[varName] = value
	default:
		varFromFile[varName] = string(byteConfig)
	}

	return varFromFile, nil
}
