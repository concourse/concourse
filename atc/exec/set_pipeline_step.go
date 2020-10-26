package exec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"sigs.k8s.io/yaml"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/configvalidate"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec/artifact"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/vars"
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
	client          worker.Client
}

func NewSetPipelineStep(
	planID atc.PlanID,
	plan atc.SetPipelinePlan,
	metadata StepMetadata,
	delegateFactory SetPipelineStepDelegateFactory,
	teamFactory db.TeamFactory,
	buildFactory db.BuildFactory,
	client worker.Client,
) Step {
	return &SetPipelineStep{
		planID:          planID,
		plan:            plan,
		metadata:        metadata,
		delegateFactory: delegateFactory,
		teamFactory:     teamFactory,
		buildFactory:    buildFactory,
		client:          client,
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

	stdout := delegate.Stdout()
	stderr := delegate.Stderr()

	fmt.Fprintln(stderr, "\x1b[1;33mWARNING: the set_pipeline step is experimental and subject to change!\x1b[0m")
	fmt.Fprintln(stderr, "")
	fmt.Fprintln(stderr, "\x1b[33mfollow RFC #31 for updates: https://github.com/concourse/rfcs/pull/31\x1b[0m")
	fmt.Fprintln(stderr, "")

	var pipelineRef atc.PipelineRef
	var teamName string
	if step.plan.Name == "self" {
		fmt.Fprintln(stderr, "\x1b[1;33mWARNING: 'set_pipeline: self' is experimental and may be removed in the future!\x1b[0m")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "\x1b[33mcontribute to discussion #5732 with feedback: https://github.com/concourse/concourse/discussions/5732\x1b[0m")
		fmt.Fprintln(stderr, "")

		pipelineRef.Name = step.metadata.PipelineName
		pipelineRef.InstanceVars = step.metadata.PipelineInstanceVars
		// self must be set to current team, thus ignore team.
		teamName = ""
	} else {
		pipelineRef.Name = step.plan.Name
		instanceVars, err := interpolateMapStringAny(step.plan.InstanceVars, state)
		if err != nil {
			return false, err
		}
		pipelineRef.InstanceVars = instanceVars
		teamName, err = step.plan.Team.Interpolate(state)
		if err != nil {
			return false, err
		}
	}

	source := setPipelineSource{
		repo:   state.ArtifactRepository(),
		client: step.client,

		file:         step.plan.File,
		vars:         step.plan.Vars,
		varFiles:     step.plan.VarFiles,
		instanceVars: pipelineRef.InstanceVars,
	}

	err := source.Validate()
	if err != nil {
		return false, err
	}

	atcConfig, err := source.FetchPipelineConfig(lagerctx.NewContext(ctx, logger), state)
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
	if teamName == "" {
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

		targetTeam, found, err := step.teamFactory.FindTeam(teamName)
		if err != nil {
			return false, err
		}
		if !found {
			return false, fmt.Errorf("team %s not found", teamName)
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
	repo   *build.Repository
	client worker.Client

	file         vars.String
	vars         map[vars.String]vars.Any
	varFiles     []vars.String
	instanceVars atc.InstanceVars
}

func (s setPipelineSource) Validate() error {
	if s.file == "" {
		return errors.New("file is not specified")
	}

	if !atc.EnablePipelineInstances && s.instanceVars != nil {
		return errors.New("support for `instance_vars` is disabled")
	}

	return nil
}

// FetchConfig streams pipeline config file and var files from other resources
// and construct an atc.Config object
func (s setPipelineSource) FetchPipelineConfig(ctx context.Context, resolver vars.Resolver) (atc.Config, error) {
	file, err := s.file.Interpolate(resolver)
	if err != nil {
		return atc.Config{}, err
	}
	config, err := s.fetchFile(ctx, file)
	if err != nil {
		return atc.Config{}, err
	}

	staticVars := []vars.Variables{}
	vars_, err := interpolateMapStringAny(s.vars, resolver)
	if err != nil {
		return atc.Config{}, err
	}
	if len(vars_) > 0 {
		staticVars = append(staticVars, vars.StaticVariables(vars_))
	}
	for _, lvf := range s.varFiles {
		varFile, err := lvf.Interpolate(resolver)
		if err != nil {
			return atc.Config{}, err
		}
		bytes, err := s.fetchFile(ctx, varFile)
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

	if len(s.instanceVars) > 0 {
		staticVars = append(staticVars, vars.StaticVariables(s.instanceVars))
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

func (s setPipelineSource) fetchFile(ctx context.Context, path string) ([]byte, error) {
	segs := strings.SplitN(path, "/", 2)
	if len(segs) != 2 {
		return nil, UnspecifiedArtifactSourceError{path}
	}

	artifactName := segs[0]
	filePath := segs[1]

	stream, err := s.retrieveFromArtifact(ctx, artifactName, filePath)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	byteConfig, err := ioutil.ReadAll(stream)
	if err != nil {
		return nil, err
	}

	return byteConfig, nil
}

func (s setPipelineSource) retrieveFromArtifact(ctx context.Context, name, file string) (io.ReadCloser, error) {
	art, found := s.repo.ArtifactFor(build.ArtifactName(name))
	if !found {
		return nil, UnknownArtifactSourceError{build.ArtifactName(name), file}
	}

	logger := lagerctx.FromContext(ctx)
	stream, err := s.client.StreamFileFromArtifact(ctx, logger, art, file)
	if err != nil {
		if err == baggageclaim.ErrFileNotFound {
			return nil, artifact.FileNotFoundError{
				Name:     name,
				FilePath: file,
			}
		}

		return nil, err
	}

	return stream, nil
}

func interpolateMapStringAny(m map[vars.String]vars.Any, resolver vars.Resolver) (map[string]interface{}, error) {
	if m == nil {
		return nil, nil
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		kk, err := k.Interpolate(resolver)
		if err != nil {
			return nil, err
		}
		vv, err := vars.Interpolate(v, resolver)
		if err != nil {
			return nil, err
		}
		out[kk] = vv
	}
	return out, nil
}
