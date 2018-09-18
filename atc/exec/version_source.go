package exec

import (
	"errors"

	"github.com/concourse/atc"
)

func NewVersionSourceFromPlan(getPlan *atc.GetPlan) VersionSource {
	if getPlan.Version != nil {
		return &StaticVersionSource{
			version: *getPlan.Version,
		}
	} else if getPlan.VersionFrom != nil {
		return &PutStepVersionSource{
			planID: *getPlan.VersionFrom,
		}
	} else {
		return &EmptyVersionSource{}
	}
}

type VersionSource interface {
	Version(RunState) (atc.Version, error)
}

type StaticVersionSource struct {
	version atc.Version
}

func (p *StaticVersionSource) Version(RunState) (atc.Version, error) {
	return p.version, nil
}

var ErrPutStepVersionMissing = errors.New("version is missing from put step")

type PutStepVersionSource struct {
	planID atc.PlanID
}

func (p *PutStepVersionSource) Version(state RunState) (atc.Version, error) {
	var info VersionInfo
	if !state.Result(p.planID, &info) {
		return atc.Version{}, ErrPutStepVersionMissing
	}

	return info.Version, nil
}

type EmptyVersionSource struct{}

func (p *EmptyVersionSource) Version(RunState) (atc.Version, error) {
	return atc.Version{}, nil
}
