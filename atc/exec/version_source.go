package exec

import (
	"errors"

	"github.com/concourse/concourse/atc"
)

func NewVersionSourceFromPlan(getPlan *atc.GetPlan) VersionSource {
	if getPlan.Version != nil {
		return &StaticVersionSource{
			version: *getPlan.Version,
			space:   getPlan.Space,
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
	SpaceVersion(RunState) (atc.Version, atc.Space, error)
}

type StaticVersionSource struct {
	version atc.Version
	space   atc.Space
}

func (p *StaticVersionSource) SpaceVersion(RunState) (atc.Version, atc.Space, error) {
	return p.version, p.space, nil
}

var ErrPutStepVersionMissing = errors.New("version is missing from put step")

type PutStepVersionSource struct {
	planID atc.PlanID
}

func (p *PutStepVersionSource) SpaceVersion(state RunState) (atc.Version, atc.Space, error) {
	var info VersionInfo
	if !state.Result(p.planID, &info) {
		return atc.Version{}, "", ErrPutStepVersionMissing
	}

	return info.Version, info.Space, nil
}

type EmptyVersionSource struct{}

func (p *EmptyVersionSource) SpaceVersion(RunState) (atc.Version, atc.Space, error) {
	return atc.Version{}, "", nil
}
