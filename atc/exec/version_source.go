package exec

import (
	"errors"

	"github.com/concourse/concourse/atc"
)

func NewVersionSourceFromPlan(getPlan *atc.GetPlan) VersionSource {
	if getPlan.Version != nil {
		return &StaticVersionSource{
			version: *getPlan.Version,
		}
	} else if getPlan.VersionFrom != nil {
		return &DynamicVersionSource{
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

var ErrResultMissing = errors.New("version is missing from previous step")

type DynamicVersionSource struct {
	planID atc.PlanID
}

func (p *DynamicVersionSource) Version(state RunState) (atc.Version, error) {
	var version atc.Version
	if !state.Result(p.planID, &version) {
		return atc.Version{}, ErrResultMissing
	}

	return version, nil
}

type EmptyVersionSource struct{}

func (p *EmptyVersionSource) Version(RunState) (atc.Version, error) {
	return atc.Version{}, nil
}
