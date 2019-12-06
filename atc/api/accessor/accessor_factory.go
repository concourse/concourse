package accessor

import (
	"code.cloudfoundry.org/lager"
	"crypto/rsa"
	"fmt"
	"net/http"
	"strings"

	jwt "github.com/dgrijalva/jwt-go"
)

//go:generate counterfeiter . AccessFactory

type AccessFactory interface {
	ActionRoleMapModifier
	ActionRoleMap

	Create(*http.Request, string) Access
}

type accessFactory struct {
	publicKey      *rsa.PublicKey
	rolesActionMap map[string]string
}

func NewAccessFactory(key *rsa.PublicKey) AccessFactory {

	factory := accessFactory{
		publicKey:      key,
		rolesActionMap: map[string]string{},
	}

	// Copy rolesActionMap
	for k, v := range requiredRoles {
		factory.rolesActionMap[k] = v
	}

	return &factory
}

func (a *accessFactory) Create(r *http.Request, action string) Access {

	header := r.Header.Get("Authorization")
	if header == "" {
		return &access{nil, action, a}
	}

	if len(header) < 7 || strings.ToUpper(header[0:6]) != "BEARER" {
		return &access{&jwt.Token{}, action, a}
	}

	token, err := jwt.Parse(header[7:], a.validate)
	if err != nil {
		return &access{&jwt.Token{}, action, a}
	}

	return &access{token, action, a}
}

func (a *accessFactory) validate(token *jwt.Token) (interface{}, error) {

	if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
		return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
	}

	return a.publicKey, nil
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
