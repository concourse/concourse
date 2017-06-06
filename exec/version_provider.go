package exec

import (
	"errors"

	"github.com/concourse/atc"
)

func NewVersionProviderFromPlan(
	getPlan *atc.GetPlan,
	putActions map[atc.PlanID]*PutAction,
) VersionProvider {
	if getPlan.Version != nil {
		return &StaticVersionProvider{
			Version: *getPlan.Version,
		}
	} else if getPlan.VersionFrom != nil {
		return &PutActionVersionProvider{
			Action: putActions[*getPlan.VersionFrom],
		}
	} else {
		return &EmptyVersionProvider{}
	}
}

type VersionProvider interface {
	GetVersion() (atc.Version, error)
}

type StaticVersionProvider struct {
	Version atc.Version
}

func (p *StaticVersionProvider) GetVersion() (atc.Version, error) {
	return p.Version, nil
}

var ErrPutActionVersionMissing = errors.New("version is missing from put action")

type PutActionVersionProvider struct {
	Action PutResultAction
}

func (p *PutActionVersionProvider) GetVersion() (atc.Version, error) {
	versionInfo, present := p.Action.Result()
	if !present {
		return atc.Version{}, ErrPutActionVersionMissing
	}

	return versionInfo.Version, nil
}

type EmptyVersionProvider struct{}

func (p *EmptyVersionProvider) GetVersion() (atc.Version, error) {
	return atc.Version{}, nil
}
