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
	"github.com/concourse/concourse/atc/creds"
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
	planID       atc.PlanID
	plan         atc.SetPipelinePlan
	metadata     StepMetadata
	delegate     BuildStepDelegate
	teamFactory  db.TeamFactory
	buildFactory db.BuildFactory
	client       worker.Client
	succeeded    bool
}

func NewSetPipelineStep(
	planID atc.PlanID,
	plan atc.SetPipelinePlan,
	metadata StepMetadata,
	delegate BuildStepDelegate,
	teamFactory db.TeamFactory,
	buildFactory db.BuildFactory,
	client worker.Client,
) Step {
	return &SetPipelineStep{
		planID:       planID,
		plan:         plan,
		metadata:     metadata,
		delegate:     delegate,
		teamFactory:  teamFactory,
		buildFactory: buildFactory,
		client:       client,
	}
}

func (step *SetPipelineStep) Run(ctx context.Context, state RunState) error {
	ctx, span := tracing.StartSpan(ctx, "set_pipeline", tracing.Attrs{
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

func (step *SetPipelineStep) run(ctx context.Context, state RunState) error {
	logger := lagerctx.FromContext(ctx)
	logger = logger.Session("set-pipeline-step", lager.Data{
		"step-name": step.plan.Name,
		"job-id":    step.metadata.JobID,
	})

	step.delegate.Initializing(logger)

	variables := step.delegate.Variables()
	interpolatedPlan, err := creds.NewSetPipelinePlan(variables, step.plan).Evaluate()
	if err != nil {
		return err
	}
	step.plan = interpolatedPlan

	stdout := step.delegate.Stdout()
	stderr := step.delegate.Stderr()

	fmt.Fprintln(stderr, "\x1b[1;33mWARNING: the set_pipeline step is experimental and subject to change!\x1b[0m")
	fmt.Fprintln(stderr, "")
	fmt.Fprintln(stderr, "\x1b[33mfollow RFC #31 for updates: https://github.com/concourse/rfcs/pull/31\x1b[0m")
	fmt.Fprintln(stderr, "")

	if step.plan.Name == "self" {
		fmt.Fprintln(stderr, "\x1b[1;33mWARNING: 'set_pipeline: self' is experimental and may be removed in the future!\x1b[0m")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "\x1b[33mcontribute to discussion #5732 with feedback: https://github.com/concourse/concourse/discussions/5732\x1b[0m")
		fmt.Fprintln(stderr, "")

		step.plan.Name = step.metadata.PipelineName
		// self must be set to current team, thus ignore team.
		step.plan.Team = ""
	}

	source := setPipelineSource{
		ctx:    ctx,
		logger: logger,
		step:   step,
		repo:   state.ArtifactRepository(),
		client: step.client,
	}

	err = source.Validate()
	if err != nil {
		return err
	}

	atcConfig, err := source.FetchPipelineConfig()
	if err != nil {
		return err
	}

	step.delegate.Starting(logger)

	warnings, errors := configvalidate.Validate(atcConfig)
	for _, warning := range warnings {
		fmt.Fprintf(stderr, "WARNING: %s\n", warning.Message)
	}

	if len(errors) > 0 {
		fmt.Fprintln(step.delegate.Stderr(), "invalid pipeline:")

		for _, e := range errors {
			fmt.Fprintf(stderr, "- %s", e)
		}

		step.delegate.Finished(logger, false)
		return nil
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
			return err
		}
		if !found {
			return fmt.Errorf("team %s not found", step.metadata.TeamName)
		}

		targetTeam, found, err := step.teamFactory.FindTeam(step.plan.Team)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("team %s not found", step.plan.Team)
		}

		permitted := false
		if targetTeam.ID() == currentTeam.ID() {
			permitted = true
		}
		if currentTeam.Admin() {
			permitted = true
		}
		if !permitted {
			return fmt.Errorf(
				"only %s team can set another team's pipeline",
				atc.DefaultTeamName,
			)
		}

		team = targetTeam
	}

	fromVersion := db.ConfigVersion(0)
	pipeline, found, err := team.Pipeline(step.plan.Name)
	if err != nil {
		return err
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

	diffExists := existingConfig.Diff(stdout, atcConfig)
	if !diffExists {
		logger.Debug("no-diff")

		fmt.Fprintf(stdout, "no diff found.\n")
		err := pipeline.SetParentIDs(step.metadata.JobID, step.metadata.BuildID)
		if err != nil {
			return err
		}
		step.succeeded = true
		step.delegate.Finished(logger, true)
		return nil
	}

	fmt.Fprintf(stdout, "setting pipeline: %s\n", step.plan.Name)
	parentBuild, found, err := step.buildFactory.Build(step.metadata.BuildID)
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("set_pipeline step not attached to a buildID")
	}

	pipeline, _, err = parentBuild.SavePipeline(step.plan.Name, team.ID(), atcConfig, fromVersion, false)
	if err != nil {
		if err == db.ErrSetByNewerBuild {
			fmt.Fprintln(stderr, "\x1b[1;33mWARNING: the pipeline was not saved because it was already saved by a newer build\x1b[0m")
			step.succeeded = true
			step.delegate.Finished(logger, true)
			return nil
		}
		return err
	}

	fmt.Fprintf(stdout, "done\n")
	logger.Info("saved-pipeline", lager.Data{"team": team.Name(), "pipeline": pipeline.Name()})
	step.succeeded = true
	step.delegate.Finished(logger, true)

	return nil
}

func (step *SetPipelineStep) Succeeded() bool {
	return step.succeeded
}

type setPipelineSource struct {
	ctx    context.Context
	logger lager.Logger
	repo   *build.Repository
	step   *SetPipelineStep
	client worker.Client
}

func (s setPipelineSource) Validate() error {
	if s.step.plan.File == "" {
		return errors.New("file is not specified")
	}

	return nil
}

// FetchConfig streams pipeline config file and var files from other resources
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

	byteConfig, err := ioutil.ReadAll(stream)
	if err != nil {
		return nil, err
	}

	return byteConfig, nil
}

func (s setPipelineSource) retrieveFromArtifact(name, file string) (io.ReadCloser, error) {
	art, found := s.repo.ArtifactFor(build.ArtifactName(name))
	if !found {
		return nil, UnknownArtifactSourceError{build.ArtifactName(name), file}
	}

	stream, err := s.client.StreamFileFromArtifact(s.ctx, s.logger, art, file)
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
