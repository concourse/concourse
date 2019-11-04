package exec

import (
	"context"
	"errors"
	"fmt"
	"github.com/concourse/baggageclaim"
	"io/ioutil"
	"sigs.k8s.io/yaml"
	"strings"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/exec/artifact"
	"github.com/concourse/concourse/vars"
)

//go:generate counterfeiter . SetPipelineDelegate

type SetPipelineDelegate interface {
	BuildStepDelegate

	Initializing(lager.Logger)
	Starting(lager.Logger)
	Finished(lager.Logger, ExitStatus, db.ConfigVersion, db.ConfigVersion)

	FindTeam() (db.Team, bool, error)
}

// SetPipelineStep sets a pipeline to current team. If pipeline_name specified
// is "self", then it will self set the current pipeline. This step takes pipeline
// configure file and var files from some resource in the pipeline, like git.
type SetPipelineStep struct {
	planID      atc.PlanID
	plan        atc.SetPipelinePlan
	metadata    StepMetadata
	delegate    SetPipelineDelegate
	lockFactory lock.LockFactory
	succeeded   bool
}

func NewSetPipelineStep(
	planID atc.PlanID,
	plan atc.SetPipelinePlan,
	metadata StepMetadata,
	delegate SetPipelineDelegate,
	lockFactory lock.LockFactory,
) Step {
	return &SetPipelineStep{
		planID:   planID,
		plan:     plan,
		metadata: metadata,
		delegate:    delegate,
		lockFactory: lockFactory,
	}
}

func (step *SetPipelineStep) Run(ctx context.Context, state RunState) error {
	logger := lagerctx.FromContext(ctx)
	logger = logger.Session("set-pipeline-step", lager.Data{
		"step-name": step.plan.Name,
		"job-id":    step.metadata.JobID,
	})

	step.delegate.Initializing(logger)

	variables := step.delegate.Variables()
	params, err := creds.NewParams(variables, step.plan.Params).Evaluate()
	if err != nil {
		return err
	}

	spParams, err := atc.NewSetPipelineParams(params)
	if err != nil {
		return err
	}

	repository := state.Artifacts()
	artifactSource := setPipelineArtifactSource{
		ctx:    ctx,
		logger: logger,
		params: spParams,
		repo:   repository,
	}

	atcConfig, err := artifactSource.FetchConfig()
	if err != nil {
		return err
	}

	step.delegate.Starting(logger)

	stdout := step.delegate.Stdout()

	if spParams.PipelineName == "self" {
		spParams.PipelineName = step.metadata.PipelineName
	}

	team, found, err := step.delegate.FindTeam()
	if err != nil {
		logger.Error("failed-to-find-team", err)
		return err
	}
	if !found {
		err = fmt.Errorf("team not found")
		logger.Error("failed-to-find-team", err)
		return err
	}

	fromVersion := db.ConfigVersion(0)
	pipeline, found, err := team.Pipeline(spParams.PipelineName)
	if err != nil {
		return nil
	}

	var existingConfig atc.Config
	if !found {
		existingConfig = atc.Config{}
	} else {
		fromVersion = pipeline.ConfigVersion()
		existingConfig, err = pipeline.Config()
		if err != nil {
			return err
		}
	}

	diffExists := existingConfig.Diff(stdout, *atcConfig)
	if !diffExists {
		logger.Debug("no-diff")

		if spParams.FailWhenNoDiff {
			return errors.New("no diff found")
		}

		fmt.Fprintf(stdout, "No diff found.\n")
		step.succeeded = true
		step.delegate.Finished(logger, ExitStatus(0), fromVersion, fromVersion)
		return nil
	}

	fmt.Fprintf(stdout, "Updating the pipeline.\n")
	pipeline, _, err = team.SavePipeline(spParams.PipelineName, *atcConfig, fromVersion, false)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Done successfully.\n")
	logger.Info("saved-pipeline", lager.Data{"team": team.Name(), "pipeline": pipeline.Name()})
	step.succeeded = true
	step.delegate.Finished(logger, ExitStatus(0), fromVersion, pipeline.ConfigVersion())

	return nil
}

func (step *SetPipelineStep) Succeeded() bool {
	return step.succeeded
}

type setPipelineArtifactSource struct {
	params atc.SetPipelineParams
	ctx    context.Context
	logger lager.Logger
	repo   *artifact.Repository
}

// streamInBytes streams a file from other resource and returns a byte array.
func (s setPipelineArtifactSource) streamInBytes(path string) ([]byte, error) {
	segs := strings.SplitN(path, "/", 2)
	if len(segs) != 2 {
		return nil, UnspecifiedArtifactSourceError{path}
	}

	sourceName := artifact.Name(segs[0])
	filePath := segs[1]

	source, found := s.repo.SourceFor(sourceName)
	if !found {
		return nil, UnknownArtifactSourceError{sourceName, path}
	}

	stream, err := source.StreamFile(s.ctx, s.logger, filePath)
	if err != nil {
		if err == baggageclaim.ErrFileNotFound {
			return nil, fmt.Errorf("task config '%s/%s' not found", sourceName, filePath)
		}
		return nil, err
	}

	defer stream.Close()

	byteConfig, err := ioutil.ReadAll(stream)
	if err != nil {
		return nil, err
	}

	return byteConfig, nil
}

// FetchConfig streams pipeline configure file and var files from other resources
// and construct an atc.Config object.
func (s setPipelineArtifactSource) FetchConfig() (*atc.Config, error) {
	config, err := s.streamInBytes(s.params.Config)
	if err != nil {
		return nil, err
	}

	staticVarss := []vars.Variables{}
	if len(s.params.Var) > 0 {
		staticVarss = append(staticVarss, vars.StaticVariables(s.params.Var))
	}
	for _, lvf := range s.params.LoadVarsFrom {
		bytes, err := s.streamInBytes(lvf)
		if err != nil {
			return nil, err
		}

		sv := vars.StaticVariables{}
		err = yaml.Unmarshal(bytes, &sv)
		if err != nil {
			return nil, err
		}

		staticVarss = append(staticVarss, sv)
	}

	if len(staticVarss) > 0 {
		config, err = vars.NewTemplateResolver(config, staticVarss).Resolve(false, false)
		if err != nil {
			return nil, err
		}
	}

	atcConfig := atc.Config{}
	err = yaml.Unmarshal(config, &atcConfig)
	if err != nil {
		return nil, err
	}

	return &atcConfig, nil
}
