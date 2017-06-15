package exec

import (
	"errors"

	"github.com/concourse/atc"
)

func NewVersionSourceFromPlan(
	getPlan *atc.GetPlan,
	putActions map[atc.PlanID]*PutAction,
) VersionSource {
	if getPlan.Version != nil {
		return &StaticVersionSource{
			Version: *getPlan.Version,
		}
	} else if getPlan.VersionFrom != nil {
		return &PutActionVersionSource{
			Action: putActions[*getPlan.VersionFrom],
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

var ErrPutActionVersionMissing = errors.New("version is missing from put action")

type PutActionVersionSource struct {
	Action *PutAction
}

func (p *PutActionVersionSource) GetVersion() (atc.Version, error) {
	versionInfo := p.Action.VersionInfo()
	if versionInfo.Version == nil {
		return atc.Version{}, ErrPutActionVersionMissing
	}

	return versionInfo.Version, nil
}

type EmptyVersionSource struct{}

func (p *EmptyVersionSource) GetVersion() (atc.Version, error) {
	return atc.Version{}, nil
}
