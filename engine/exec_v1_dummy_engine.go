package engine

import (
	"errors"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

type execV1DummyEngine struct{}

func NewExecV1DummyEngine() Engine {
	return execV1DummyEngine{}
}

func (execV1DummyEngine) Name() string {
	return "exec.v1"
}

func (execV1DummyEngine) CreateBuild(logger lager.Logger, model db.Build, plan atc.Plan) (Build, error) {
	return nil, errors.New("dummy engine does not support new builds")
}

func (execV1DummyEngine) LookupBuild(logger lager.Logger, model db.Build) (Build, error) {
	return execV1DummyBuild{}, nil
}

type execV1DummyBuild struct {
}

func (execV1DummyBuild) Metadata() string {
	return ""
}

func (execV1DummyBuild) PublicPlan(lager.Logger) (atc.PublicBuildPlan, bool, error) {
	return atc.PublicBuildPlan{
		Schema: execEngineName,
		Plan:   nil,
	}, true, nil
}

func (execV1DummyBuild) Abort(lager.Logger) error {
	return nil
}

func (execV1DummyBuild) Resume(logger lager.Logger) {
}
