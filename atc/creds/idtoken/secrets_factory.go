package idtoken

import (
	"github.com/concourse/concourse/atc/creds"
)

type SecretsFactory struct {
	TokenGenerator *TokenGenerator
}

func NewSecretsFactory(TokenGenerator *TokenGenerator) *SecretsFactory {
	return &SecretsFactory{
		TokenGenerator: TokenGenerator,
	}
}

func (factory *SecretsFactory) NewSecrets() creds.Secrets {
	return &Secrets{
		TokenGenerator: factory.TokenGenerator,
	}
}
