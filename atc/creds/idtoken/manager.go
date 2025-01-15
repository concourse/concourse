package idtoken

import (
	"encoding/json"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager/v3"

	"github.com/concourse/concourse/atc/creds"
)

type Manager struct {
	Config map[string]interface{}

	TokenGenerator TokenGenerator
}

func (manager *Manager) Init(log lager.Logger) error {
	aud := make([]string, 0, 1)
	if audList, ok := manager.Config["aud"].([]interface{}); ok {
		for _, e := range audList {
			if audience, ok := e.(string); ok {
				aud = append(aud, audience)
			}
		}
	}
	if len(aud) == 0 {
		aud = append(aud, defaultAudience...)
	}

	ttl := defaultTTL
	var err error
	ttlString, ok := manager.Config["ttl"].(string)
	if ok {
		ttl, err = time.ParseDuration(ttlString)
		if err != nil {
			return fmt.Errorf("invalid idtoken provider config: invalid TTL value: %w", err)
		}
	}

	signKey, err := GenerateNewKey()
	if err != nil {
		return err
	}

	manager.TokenGenerator = TokenGenerator{
		Issuer:    "https://test",
		Audiences: aud,
		TTL:       ttl,
		Key:       signKey,
	}

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
	return false
}

func (manager Manager) Validate() error {
	return nil
}

func (manager Manager) Health() (*creds.HealthResponse, error) {
	return &creds.HealthResponse{
		Method: "noop",
	}, nil
}

func (manager Manager) Close(logger lager.Logger) {

}

func (manager Manager) NewSecretsFactory(logger lager.Logger) (creds.SecretsFactory, error) {
	return NewSecretsFactory(&manager.TokenGenerator), nil
}
