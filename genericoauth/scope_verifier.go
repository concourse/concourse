package genericoauth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"golang.org/x/oauth2"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/skymarshal/verifier"
	"github.com/dgrijalva/jwt-go"
)

type ScopeVerifier struct {
	scope string
}

func NewScopeVerifier(
	scope string,
) verifier.Verifier {
	return ScopeVerifier{
		scope: scope,
	}
}

type GenericOAuthToken struct {
	Scopes []string `json:"scope"`
}

func (verifier ScopeVerifier) Verify(logger lager.Logger, httpClient *http.Client) (bool, error) {
	oauth2Transport, ok := httpClient.Transport.(*oauth2.Transport)
	if !ok {
		return false, errors.New("httpClient transport must be of type oauth2.Transport")
	}

	token, err := oauth2Transport.Source.Token()
	if err != nil {
		return false, err
	}

	tokenParts := strings.Split(token.AccessToken, ".")
	if len(tokenParts) < 2 {
		return false, errors.New("access token contains an invalid number of segments")
	}

	decodedClaims, err := jwt.DecodeSegment(tokenParts[1])
	if err != nil {
		return false, err
	}

	var oauthToken GenericOAuthToken
	err = json.Unmarshal(decodedClaims, &oauthToken)
	if err != nil {
		return false, err
	}

	if len(oauthToken.Scopes) == 0 {
		return false, errors.New("user has no assigned scopes in access token")
	}

	for _, userScope := range oauthToken.Scopes {
		if userScope == verifier.scope {
			return true, nil
		}
	}

	logger.Info("does-not-have-scope", lager.Data{
		"have": oauthToken.Scopes,
		"want": verifier.scope,
	})

	return false, nil
}
