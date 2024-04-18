package exec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagerctx"
	"sigs.k8s.io/yaml"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/configvalidate"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/vars"
	"github.com/concourse/concourse/worker/baggageclaim"
)

// SetPipelineStep sets a pipeline to current team. This step takes pipeline
// configure file and var files from some resource in the pipeline, like git.
type SetPipelineStep struct {
	planID          atc.PlanID
	plan            atc.SetPipelinePlan
	metadata        StepMetadata
	delegateFactory SetPipelineStepDelegateFactory
	teamFactory     db.TeamFactory
	buildFactory    db.BuildFactory
	streamer        Streamer
}

func NewSetPipelineStep(
	planID atc.PlanID,
	plan atc.SetPipelinePlan,
	metadata StepMetadata,
	delegateFactory SetPipelineStepDelegateFactory,
	teamFactory db.TeamFactory,
	buildFactory db.BuildFactory,
	streamer Streamer,
) Step {
	return &SetPipelineStep{
		planID:          planID,
		plan:            plan,
		metadata:        metadata,
		delegateFactory: delegateFactory,
		teamFactory:     teamFactory,
		buildFactory:    buildFactory,
		streamer:        streamer,
	}
}

func (step *SetPipelineStep) Run(ctx context.Context, state RunState) (bool, error) {
	delegate := step.delegateFactory.SetPipelineStepDelegate(state)
	ctx, span := delegate.StartSpan(ctx, "set_pipeline", tracing.Attrs{
		"name": step.plan.Name,
	})

	ok, err := step.run(ctx, state, delegate)
	tracing.End(span, err)

	return ok, err
}

func (step *SetPipelineStep) run(ctx context.Context, state RunState, delegate SetPipelineStepDelegate) (bool, error) {
	logger := lagerctx.FromContext(ctx)
	logger = logger.Session("set-pipeline-step", lager.Data{
		"step-name": step.plan.Name,
		"job-id":    step.metadata.JobID,
	})

	delegate.Initializing(logger)

	interpolatedPlan, err := creds.NewSetPipelinePlan(state, step.plan).Evaluate()
	if err != nil {
		return false, err
	}
	step.plan = interpolatedPlan

	stdout := delegate.Stdout()
	stderr := delegate.Stderr()

	if step.plan.Name == "self" {
		fmt.Fprintln(stderr, "\x1b[1;33mWARNING: 'set_pipeline: self' is experimental and may be removed in the future!\x1b[0m")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "\x1b[33mcontribute to discussion #5732 with feedback: https://github.com/concourse/concourse/discussions/5732\x1b[0m")
		fmt.Fprintln(stderr, "")

		step.plan.Name = step.metadata.PipelineName
		step.plan.InstanceVars = step.metadata.PipelineInstanceVars
		// self must be set to current team, thus ignore team.
		step.plan.Team = ""
	}

	source := setPipelineSource{
		ctx:      ctx,
		logger:   logger,
		step:     step,
		repo:     state.ArtifactRepository(),
		streamer: step.streamer,
	}

	err = source.Validate()
	if err != nil {
		return false, err
	}

	atcConfig, err := source.FetchPipelineConfig()
	if err != nil {
		return false, err
	}

	delegate.Starting(logger)

	warnings, errors := configvalidate.Validate(atcConfig)
	for _, warning := range warnings {
		fmt.Fprintf(stderr, "WARNING: %s\n", warning.Message)
	}

	if len(errors) > 0 {
		fmt.Fprintln(delegate.Stderr(), "invalid pipeline:")

		for _, e := range errors {
			fmt.Fprintf(stderr, "- %s", e)
		}

		delegate.Finished(logger, false)
		return false, nil
	}

	var team db.Team
	if step.plan.Team == "" {
		team = step.teamFactory.GetByID(step.metadata.TeamID)
	} else {
		fmt.Fprintln(stderr, "\x1b[1;33mWARNING: specifying the team in a set_pipeline step is experimental and may be removed in the future!\x1b[0m")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "\x1b[33mcontribute to discussion #5731 with feedback: https://github.com/concourse/concourse/discussions/5731\x1b[0m")
		fmt.Fprintln(stderr, "")

		currentTeam, found, err := step.teamFactory.FindTeam(step.metadata.TeamName)
		if err != nil {
			return false, err
		}
		if !found {
			return false, fmt.Errorf("team %s not found", step.metadata.TeamName)
		}

		targetTeam, found, err := step.teamFactory.FindTeam(step.plan.Team)
		if err != nil {
			return false, err
		}
		if !found {
			return false, fmt.Errorf("team %s not found", step.plan.Team)
		}

		permitted := false
		if targetTeam.ID() == currentTeam.ID() {
			permitted = true
		}
		if currentTeam.Admin() {
			permitted = true
		}
		if !permitted {
			return false, fmt.Errorf(
				"only %s team can set another team's pipeline",
				atc.DefaultTeamName,
			)
		}

		team = targetTeam
	}

	pipelineRef := atc.PipelineRef{
		Name:         step.plan.Name,
		InstanceVars: step.plan.InstanceVars,
	}
	pipeline, found, err := team.Pipeline(pipelineRef)
	if err != nil {
		return false, err
	}

	fromVersion := db.ConfigVersion(0)
	var existingConfig atc.Config
	if !found {
		existingConfig = atc.Config{}
	} else {
		fromVersion = pipeline.ConfigVersion()
		existingConfig, err = pipeline.Config()
		if err != nil {
			return false, err
		}
	}

	diffExists := existingConfig.Diff(stdout, atcConfig)
	if !diffExists {
		logger.Debug("no-diff")

		fmt.Fprintf(stdout, "no changes to apply.\n")

		if found {
			err := pipeline.SetParentIDs(step.metadata.JobID, step.metadata.BuildID)
			if err != nil {
				return false, err
			}
		}

		delegate.SetPipelineChanged(logger, false)
		delegate.Finished(logger, true)
		return true, nil
	}

	err = delegate.CheckRunSetPipelinePolicy(&atcConfig)
	if err != nil {
		return false, err
	}

	fmt.Fprintf(stdout, "setting pipeline: %s\n", pipelineRef.String())
	delegate.SetPipelineChanged(logger, true)

	parentBuild, found, err := step.buildFactory.Build(step.metadata.BuildID)
	if err != nil {
		return false, err
	}

	if !found {
		return false, fmt.Errorf("set_pipeline step not attached to a buildID")
	}

	pipeline, _, err = parentBuild.SavePipeline(pipelineRef, team.ID(), atcConfig, fromVersion, false)
	if err != nil {
		if err == db.ErrSetByNewerBuild {
			fmt.Fprintln(stderr, "\x1b[1;33mWARNING: the pipeline was not saved because it was already saved by a newer build\x1b[0m")
			delegate.Finished(logger, true)
			return true, nil
		}
		return false, err
	}

	fmt.Fprintf(stdout, "done\n")
	logger.Info("saved-pipeline", lager.Data{"team": team.Name(), "pipeline": pipeline.Name()})
	delegate.Finished(logger, true)

	return true, nil
}

type setPipelineSource struct {
	ctx      context.Context
	logger   lager.Logger
	repo     *build.Repository
	step     *SetPipelineStep
	streamer Streamer
}

func (s setPipelineSource) Validate() error {
	if s.step.plan.File == "" {
		return errors.New("file is not specified")
	}

	if !atc.EnablePipelineInstances && s.step.plan.InstanceVars != nil {
		return errors.New("support for `instance_vars` is disabled")
	}

	return nil
}

// FetchPipelineConfig streams pipeline config file and var files from other resources
// and construct an atc.Config object
func (s setPipelineSource) FetchPipelineConfig() (atc.Config, error) {
	config, err := s.fetchPipelineBits(s.step.plan.File)
	if err != nil {
		return atc.Config{}, err
	}

	staticVars := []vars.Variables{}
	if len(s.step.plan.Vars) > 0 {
		staticVars = append(staticVars, vars.StaticVariables(s.step.plan.Vars))
	}
	for _, lvf := range s.step.plan.VarFiles {
		bytes, err := s.fetchPipelineBits(lvf)
		if err != nil {
			return atc.Config{}, err
		}

		sv := vars.StaticVariables{}
		err = yaml.Unmarshal(bytes, &sv)
		if err != nil {
			return atc.Config{}, err
		}

		staticVars = append(staticVars, sv)
	}

	if len(s.step.plan.InstanceVars) > 0 {
		iv := vars.StaticVariables{}
		for k, v := range s.step.plan.InstanceVars {
			iv[k] = v
		}
		staticVars = append(staticVars, iv)
	}

	if len(staticVars) > 0 {
		config, err = vars.NewTemplateResolver(config, staticVars).Resolve(false, false)
		if err != nil {
			return atc.Config{}, err
		}
	}

	atcConfig := atc.Config{}
	err = atc.UnmarshalConfig(config, &atcConfig)
	if err != nil {
		return atc.Config{}, err
	}

	return atcConfig, nil
}

func (s setPipelineSource) fetchPipelineBits(path string) ([]byte, error) {
	segs := strings.SplitN(path, "/", 2)
	if len(segs) != 2 {
		return nil, UnspecifiedArtifactSourceError{path}
	}

	artifactName := segs[0]
	filePath := segs[1]

	stream, err := s.retrieveFromArtifact(artifactName, filePath)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	byteConfig, err := io.ReadAll(stream)
	if err != nil {
		return nil, err
	}

	return byteConfig, nil
}

func (s setPipelineSource) retrieveFromArtifact(name, file string) (io.ReadCloser, error) {
	art, _, found := s.repo.ArtifactFor(build.ArtifactName(name))
	if !found {
		return nil, UnknownArtifactSourceError{build.ArtifactName(name), file}
	}

	stream, err := s.streamer.StreamFile(lagerctx.NewContext(s.ctx, s.logger), art, file)
	if err != nil {
		if err == baggageclaim.ErrFileNotFound {
			return nil, FileNotFoundError{
				Name:     name,
				FilePath: file,
			}
		}

		return nil, err
	}

	return stream, nil
}
