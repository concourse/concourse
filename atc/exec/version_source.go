package exec

import (
	"errors"

	"github.com/concourse/concourse/atc"
)

// ErrResultMissing is returned when a get's version_from step did not produce
// a version. The get step replaces this text with one that names the image or
// resource; the sentinel is kept for errors.Is.
var ErrResultMissing = errors.New("version is missing from previous step")

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
