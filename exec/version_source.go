package exec

import (
	"errors"

	"github.com/concourse/atc"
)

func NewVersionSourceFromPlan(
	getPlan *atc.GetPlan,
	putSteps map[atc.PlanID]*PutStep,
) VersionSource {
	if getPlan.Version != nil {
		return &StaticVersionSource{
			Version: *getPlan.Version,
		}
	} else if getPlan.VersionFrom != nil {
		return &PutStepVersionSource{
			Step: putSteps[*getPlan.VersionFrom],
		}
	} else {
		return &EmptyVersionSource{}
	}
}

type VersionSource interface {
	GetVersion() (atc.Version, error)
}

type StaticVersionSource struct {
	Version atc.Version
}

func (p *StaticVersionSource) GetVersion() (atc.Version, error) {
	return p.Version, nil
}

var ErrPutStepVersionMissing = errors.New("version is missing from put step")

type PutStepVersionSource struct {
	Step *PutStep
}

func (p *PutStepVersionSource) GetVersion() (atc.Version, error) {
	versionInfo := p.Step.VersionInfo()
	if versionInfo.Version == nil {
		return atc.Version{}, ErrPutStepVersionMissing
	}

	return versionInfo.Version, nil
}

type EmptyVersionSource struct{}

func (p *EmptyVersionSource) GetVersion() (atc.Version, error) {
	return atc.Version{}, nil
}
