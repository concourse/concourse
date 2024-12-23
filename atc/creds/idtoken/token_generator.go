package idtoken

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

type TokenGenerator struct {
	Issuer    string
	Key       jwk.Key
	Audiences []string
	TTL       time.Duration
}

func (g TokenGenerator) GenerateToken(team, pipeline string) (token string, validUntil time.Time, err error) {
	now := time.Now()
	validUntil = now.Add(g.TTL)

	unsigned, err := jwt.NewBuilder().
		Issuer(g.Issuer).
		IssuedAt(now).
		NotBefore(now).
		Audience(g.Audiences).
		Subject(generateSubject(team, pipeline)).
		Expiration(validUntil).
		JwtID(generateJTI()).
		Claim("team", team).
		Claim("pipeline", pipeline).
		Build()

	if err != nil {
		return "", time.Time{}, err
	}

	signed, err := jwt.Sign(unsigned, jwt.WithKey(jwa.RS256(), g.Key))
	if err != nil {
		return "", time.Time{}, err
	}

	return string(signed), validUntil, nil
}

func (g TokenGenerator) IsTokenStillValid(token string) (bool, time.Time, error) {
	parsed, err := jwt.Parse([]byte(token), jwt.WithKey(jwa.RS256(), g.Key))
	if err != nil {
		if strings.Contains(err.Error(), "token is expired") {
			return false, time.Time{}, nil
		}
		return false, time.Time{}, err
	}

	exp, exists := parsed.Expiration()
	if !exists {
		return false, time.Time{}, err
	}

	return true, exp, nil
}

func generateSubject(team, pipeline string) string {
	return fmt.Sprintf("%s/%s", team, pipeline)
}

func generateJTI() string {
	num, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		// should never happen
		panic(err)
	}
	return strconv.Itoa(int(num.Int64()))
}

func GenerateNewKey() (jwk.Key, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}

	key, err := jwk.Import(privateKey)
	if err != nil {
		return nil, err
	}

	key.Set("kid", generateKID())
	key.Set("iat", time.Now().Unix())

	return key, nil
}

func generateKID() string {
	num, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		// should never happen
		panic(err)
	}
	return strconv.Itoa(int(num.Int64()))
}
