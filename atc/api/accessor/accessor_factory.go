package accessor

import (
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
)

//go:generate counterfeiter . Verifier

type Verifier interface {
	Verify(*http.Request) (map[string]interface{}, error)
}

//go:generate counterfeiter . AccessFactory

type AccessFactory interface {
	ActionRoleMapModifier
	ActionRoleMap

	Create(*http.Request, string) (Access, error)
}

func NewAccessFactory(
	verifier Verifier,
	teamFactory db.TeamFactory,
	systemClaimKey string,
	systemClaimValues []string,
) AccessFactory {

	factory := accessFactory{
		verifier:          verifier,
		teamFactory:       teamFactory,
		systemClaimKey:    systemClaimKey,
		systemClaimValues: systemClaimValues,
		rolesActionMap:    map[string]string{},
	}

	// Copy rolesActionMap
	for k, v := range requiredRoles {
		factory.rolesActionMap[k] = v
	}

	return &factory
}

type accessFactory struct {
	verifier          Verifier
	teamFactory       db.TeamFactory
	systemClaimKey    string
	systemClaimValues []string
	rolesActionMap    map[string]string
}

func (a *accessFactory) Create(r *http.Request, action string) (Access, error) {

	requiredRole := a.RoleOfAction(action)

	verification := a.verify(r)

	if !verification.IsTokenValid {
		return NewAccessor(verification, requiredRole, a.systemClaimKey, a.systemClaimValues, nil), nil
	}

	teams, err := a.teamFactory.GetTeams()
	if err != nil {
		return nil, err
	}

	return NewAccessor(verification, requiredRole, a.systemClaimKey, a.systemClaimValues, teams), nil
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

func (a *accessFactory) CustomizeActionRoleMap(logger lager.Logger, customMapping CustomActionRoleMap) error {

	// Get all validate role names
	allKnownRoles := map[string]interface{}{}
	for _, roleName := range a.rolesActionMap {
		allKnownRoles[roleName] = nil
	}

	for newRole, actions := range customMapping {
		// Check if the customized role name is valid
		if _, ok := allKnownRoles[newRole]; !ok {
			return fmt.Errorf("unknown role %s", newRole)
		}

		// Update requiredRoles
		for _, action := range actions {
			if oldRole, ok := a.rolesActionMap[action]; ok {
				a.rolesActionMap[action] = newRole
				logger.Info("customize-role", lager.Data{"action": action, "oldRole": oldRole, "newRole": newRole})
			} else {
				return fmt.Errorf("unknown action %s", action)
			}
		}
	}

	return nil
}

func (a *accessFactory) RoleOfAction(action string) string {
	return a.rolesActionMap[action]
}
