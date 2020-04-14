package accessor

import (
	"github.com/concourse/concourse/atc/db"
)

func NewAccessFactory(
	systemClaimKey string,
	systemClaimValues []string,
) AccessFactory {
	return &accessFactory{
		systemClaimKey:    systemClaimKey,
		systemClaimValues: systemClaimValues,
	}
}

type accessFactory struct {
	systemClaimKey    string
	systemClaimValues []string
}

func (a *accessFactory) Create(role string, verification Verification, teams []db.Team) Access {
	return NewAccessor(verification, role, a.systemClaimKey, a.systemClaimValues, teams)
}
