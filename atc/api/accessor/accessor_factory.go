package accessor

import (
	"net/http"

	"github.com/concourse/concourse/atc/db"
)

//go:generate counterfeiter . Verifier

type Verifier interface {
	Verify(*http.Request) (map[string]interface{}, error)
}

//go:generate counterfeiter . AccessFactory

type AccessFactory interface {
	Create(*http.Request, string) (Access, error)
}

func NewAccessFactory(
	verifier Verifier,
	teamFactory db.TeamFactory,
	systemClaimKey string,
	systemClaimValues []string,
	customRoles map[string]string,
) AccessFactory {
	return &accessFactory{
		verifier:          verifier,
		teamFactory:       teamFactory,
		systemClaimKey:    systemClaimKey,
		systemClaimValues: systemClaimValues,
		customRoles:       customRoles,
	}
}

type accessFactory struct {
	verifier          Verifier
	teamFactory       db.TeamFactory
	systemClaimKey    string
	systemClaimValues []string
	customRoles       map[string]string
}

func (a *accessFactory) Create(r *http.Request, action string) (Access, error) {

	role := a.customRoles[action]

	if role == "" {
		role = DefaultRoles[action]
	}

	verification := a.verify(r)

	if !verification.IsTokenValid {
		return NewAccessor(verification, role, a.systemClaimKey, a.systemClaimValues, nil), nil
	}

	teams, err := a.teamFactory.GetTeams()
	if err != nil {
		return nil, err
	}

	return NewAccessor(verification, role, a.systemClaimKey, a.systemClaimValues, teams), nil
}

func (a *accessFactory) verify(r *http.Request) Verification {

	claims, err := a.verifier.Verify(r)
	if err != nil {
		switch err {
		case ErrVerificationNoToken:
			return Verification{HasToken: false, IsTokenValid: false}
		default:
			return Verification{HasToken: true, IsTokenValid: false}
		}
	}

	return Verification{HasToken: true, IsTokenValid: true, RawClaims: claims}
}
