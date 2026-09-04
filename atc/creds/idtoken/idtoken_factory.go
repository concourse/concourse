package idtoken

import (
	"github.com/concourse/concourse/v8/atc/creds"
)

type idtokenFactory struct {
	tokenGenerator *TokenGenerator
}

func (factory *idtokenFactory) NewSecrets() creds.Secrets {
	return &IDToken{
		TokenGenerator: factory.tokenGenerator,
	}
}
