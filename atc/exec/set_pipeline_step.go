package exec

import (
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"sigs.k8s.io/yaml"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/configvalidate"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec/artifact"
	"github.com/concourse/concourse/vars"
)

// SetPipelineStep sets a pipeline to current team. If pipeline_name specified
// is "self", then it will self set the current pipeline. This step takes pipeline
// configure file and var files from some resource in the pipeline, like git.
type SetPipelineStep struct {
	planID      atc.PlanID
	plan        atc.SetPipelinePlan
	metadata    StepMetadata
	delegate    BuildStepDelegate
	teamFactory db.TeamFactory
	succeeded   bool
}

func NewSetPipelineStep(
	planID atc.PlanID,
	plan atc.SetPipelinePlan,
	metadata StepMetadata,
	delegate BuildStepDelegate,
	teamFactory db.TeamFactory,
) Step {
	return &SetPipelineStep{
		planID:      planID,
		plan:        plan,
		metadata:    metadata,
		delegate:    delegate,
		teamFactory: teamFactory,
	}
}

func (step *SetPipelineStep) Run(ctx context.Context, state RunState) error {
	logger := lagerctx.FromContext(ctx)
	logger = logger.Session("set-pipeline-step", lager.Data{
		"step-name": step.plan.Name,
		"job-id":    step.metadata.JobID,
	})

	step.delegate.Initializing(logger)

	stdout := step.delegate.Stdout()
	stderr := step.delegate.Stderr()

	source := setPipelineSource{
		ctx:    ctx,
		logger: logger,
		step:   step,
		repo:   state.Artifacts(),
	}

	err := source.Validate()
	if err != nil {
		return err
	}

	atcConfig, err := source.FetchConfig()
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

	team := step.teamFactory.GetByID(step.metadata.TeamID)

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
		step.succeeded = true
		step.delegate.Finished(logger, true)
		return nil
	}

	fmt.Fprintf(stdout, "setting pipeline: %s\n", step.plan.Name)
	pipeline, _, err = team.SavePipeline(step.plan.Name, atcConfig, fromVersion, false)
	if err != nil {
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
	repo   *artifact.Repository
	step   *SetPipelineStep
}

// streamInBytes streams a file from other resource and returns a byte array.
func (s setPipelineSource) streamInBytes(path string) ([]byte, error) {
	stream, err := s.repo.StreamFile(s.ctx, s.logger, path)
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

// FetchConfig streams pipeline configure file and var files from other resources
// and construct an atc.Config object.
func (s setPipelineSource) FetchConfig() (atc.Config, error) {
	config, err := s.streamInBytes(s.step.plan.File)
	if err != nil {
		return atc.Config{}, err
	}

	staticVarss := []vars.Variables{}
	if len(s.step.plan.Vars) > 0 {
		staticVarss = append(staticVarss, vars.StaticVariables(s.step.plan.Vars))
	}
	for _, lvf := range s.step.plan.VarFiles {
		bytes, err := s.streamInBytes(lvf)
		if err != nil {
			return atc.Config{}, err
		}

		sv := vars.StaticVariables{}
		err = yaml.Unmarshal(bytes, &sv)
		if err != nil {
			return atc.Config{}, err
		}

		staticVarss = append(staticVarss, sv)
	}

	if len(staticVarss) > 0 {
		config, err = vars.NewTemplateResolver(config, staticVarss).Resolve(false, false)
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

func (s setPipelineSource) Validate() error {
	if s.step.plan.File == "" {
		return errors.New("file is not specified")
	}

	return nil
}
