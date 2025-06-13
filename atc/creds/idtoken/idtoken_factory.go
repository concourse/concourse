package idtoken

import (
	"github.com/concourse/concourse/atc/creds"
)

type idtokenFactory struct {
	tokenGenerator *TokenGenerator
}

func (factory *idtokenFactory) NewSecrets() creds.Secrets {
	return &IDToken{
		TokenGenerator: factory.tokenGenerator,
	}
}
