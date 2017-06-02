package execv3engine

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/engine"
)

type execV3Engine struct {
	planner *Planner
}

var engineName = "exec.v3"

func NewEngine() engine.Engine {
	return &execV3Engine{
		planner: &Planner{},
	}
}

func (e *execV3Engine) Name() string {
	return engineName
}

func (e *execV3Engine) CreateBuild(logger lager.Logger, dbBuild dbng.Build, plan atc.Plan) (engine.Build, error) {
	execPlan := e.planner.Generate(plan)
	return &build{
		metadata: buildMetadata{
			Plan: execPlan,
		},
	}, nil
}

func (e *execV3Engine) LookupBuild(logger lager.Logger, dbBuild dbng.Build) (engine.Build, error) {
	return &build{}, nil
}

func (e *execV3Engine) ReleaseAll(lager.Logger) {
}

type buildMetadata struct {
	Plan Plan
}

type build struct {
	metadata buildMetadata
}

func (b *build) Metadata() string {
	return ""
}

func (b *build) PublicPlan(lager.Logger) (atc.PublicBuildPlan, error) {
	return atc.PublicBuildPlan{
		Schema: engineName,
		Plan:   nil,
	}, nil
}

func (b *build) Abort(lager.Logger) error {
	return nil
}

func (b *build) Resume(logger lager.Logger) {
}
