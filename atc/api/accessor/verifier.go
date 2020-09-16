package accessor

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/concourse/concourse/atc/db"
	"gopkg.in/square/go-jose.v2/jwt"
)

var (
	ErrVerificationNoToken         = errors.New("token not provided")
	ErrVerificationInvalidToken    = errors.New("token provided is invalid")
	ErrVerificationTokenExpired    = errors.New("token is expired")
	ErrVerificationInvalidAudience = errors.New("token has invalid audience")
)

//go:generate counterfeiter .  AccessTokenFetcher

type AccessTokenFetcher interface {
	GetAccessToken(rawToken string) (db.AccessToken, bool, error)
}

func NewVerifier(accessTokenFetcher AccessTokenFetcher, audience []string) *verifier {
	return &verifier{
		accessTokenFetcher: accessTokenFetcher,
		audience:           audience,
	}
}

type verifier struct {
	sync.Mutex
	accessTokenFetcher AccessTokenFetcher
	audience           []string
}

func (v *verifier) Verify(r *http.Request) (map[string]interface{}, error) {

	header := r.Header.Get("Authorization")
	if header == "" {
		return nil, ErrVerificationNoToken
	}

	parts := strings.Split(header, " ")
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return nil, ErrVerificationInvalidToken
	}

	return v.verify(parts[1])
}

func (v *verifier) verify(rawToken string) (map[string]interface{}, error) {
	token, found, err := v.accessTokenFetcher.GetAccessToken(rawToken)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrVerificationInvalidToken
	}

	claims := token.Claims
	err = claims.Validate(jwt.Expected{Time: time.Now()})
	if err != nil {
		return nil, ErrVerificationTokenExpired
	}

	for _, aud := range v.audience {
		if claims.Audience.Contains(aud) {
			return claims.RawClaims, nil
		}
	}

	return nil, ErrVerificationInvalidAudience
}
