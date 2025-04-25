package idtoken

import (
	"encoding/json"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager/v3"

	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
)

type Manager struct {
	tokenGenerator TokenGenerator
}

const (
	DefaultSubjectScope = SubjectScopePipeline
	DefaultExpiresIn    = 1 * time.Hour
	MaxExpiresIn        = 24 * time.Hour
)

func NewManager(issuer string, signingKeyFactory db.SigningKeyFactory, config map[string]any) (*Manager, error) {
	m := Manager{
		tokenGenerator: TokenGenerator{
			Issuer:            issuer,
			SubjectScope:      DefaultSubjectScope,
			Audience:          nil,
			ExpiresIn:         DefaultExpiresIn,
			SigningKeyFactory: signingKeyFactory,
		},
	}
	var err error

	for key, value := range config {
		switch key {
		case "audience":
			if audList, ok := value.([]interface{}); ok {
				aud := make([]string, 0, len(audList))
				for _, e := range audList {
					if audience, ok := e.(string); ok {
						aud = append(aud, audience)
					} else {
						return nil, fmt.Errorf("invalid idtoken provider config: invalid audience value: %w", err)
					}
				}
				m.tokenGenerator.Audience = aud
			} else {
				return nil, fmt.Errorf("invalid idtoken provider config: audience must be a list of strings")
			}

		case "subject_scope":
			if subjectScopeString, ok := value.(string); ok {
				m.tokenGenerator.SubjectScope = SubjectScope(subjectScopeString)
			} else {
				return nil, fmt.Errorf("invalid idtoken provider config: subject_scope must be a string")
			}

		case "expires_in":
			if expiresInString, ok := value.(string); ok {
				m.tokenGenerator.ExpiresIn, err = time.ParseDuration(expiresInString)
				if err != nil {
					return nil, fmt.Errorf("invalid idtoken provider config: invalid expires_in value: %w", err)
				}
			} else {
				return nil, fmt.Errorf("invalid idtoken provider config: expires_in must be a string")
			}

		default:
			return nil, fmt.Errorf("invalid idtoken provider config: unknown setting: %s", key)
		}
	}

	return &m, nil
}

func (manager *Manager) Init(log lager.Logger) error {
	return nil
}

func (manager *Manager) MarshalJSON() ([]byte, error) {
	health, err := manager.Health()
	if err != nil {
		return nil, err
	}

	return json.Marshal(&map[string]interface{}{
		"health": health,
	})
}

func (manager Manager) IsConfigured() bool {
	// does not make sense to have this as the global secret source. Always return false
	return false
}

func (manager Manager) Validate() error {
	if !manager.tokenGenerator.SubjectScope.Valid() {
		return fmt.Errorf("invalid subject_scope value: %s", manager.tokenGenerator.SubjectScope)
	}
	if manager.tokenGenerator.ExpiresIn > MaxExpiresIn {
		return fmt.Errorf("expires_in must be <= %s", MaxExpiresIn.String())
	}
	return nil
}

func (manager Manager) Health() (*creds.HealthResponse, error) {
	return &creds.HealthResponse{
		Method: "noop",
	}, nil
}

func (manager Manager) Close(logger lager.Logger) {

}

func (manager Manager) GetTokenGenerator() TokenGenerator {
	return manager.tokenGenerator
}

func (manager Manager) NewSecretsFactory(logger lager.Logger) (creds.SecretsFactory, error) {
	return &idtokenFactory{
		tokenGenerator: &manager.tokenGenerator,
	}, nil
}
