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

	"github.com/concourse/concourse/atc/db"

	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
)

type SubjectScope string

const (
	SubjectScopeEmpty    SubjectScope = ""
	SubjectScopeTeam     SubjectScope = "team"
	SubjectScopePipeline SubjectScope = "pipeline"
)

func (s SubjectScope) Valid() bool {
	switch s {
	case SubjectScopeEmpty, SubjectScopeTeam, SubjectScopePipeline:
		return true
	}
	return false
}

type TokenGenerator struct {
	Issuer            string
	SigningKeyFactory db.SigningKeyFactory
	SubjectScope      SubjectScope
	Audience          []string
	ExpiresIn         time.Duration
}

func (g TokenGenerator) GenerateToken(team, pipeline string) (token string, validUntil time.Time, err error) {
	now := time.Now()
	validUntil = now.Add(g.ExpiresIn)

	// currently only RSA signatures are supported
	latestKey, err := g.SigningKeyFactory.GetNewestKey(db.SigningKeyTypeRSA)
	if err != nil {
		return "", time.Time{}, err
	}
	signingKey := jose.SigningKey{
		Algorithm: jose.RS256,
		Key:       latestKey.JWK(),
	}

	signer, err := jose.NewSigner(signingKey, &jose.SignerOptions{})
	if err != nil {
		return "", time.Time{}, err
	}

	claims := jwt.Claims{
		Issuer:   g.Issuer,
		IssuedAt: jwt.NewNumericDate(now),
		Audience: jwt.Audience(g.Audience),
		Subject:  g.generateSubject(team, pipeline),
		Expiry:   jwt.NewNumericDate(validUntil),
		ID:       generateRandomNumericString(),
	}

	customClaims := struct {
		Team     string `json:"team"`
		Pipeline string `json:"pipeline"`
	}{
		Team:     team,
		Pipeline: pipeline,
	}

	signed, err := jwt.Signed(signer).Claims(claims).Claims(customClaims).CompactSerialize()
	if err != nil {
		return "", time.Time{}, err
	}

	return signed, validUntil, nil
}

func (g TokenGenerator) generateSubject(team, pipeline string) string {
	team = escapeSlashes(team)
	pipeline = escapeSlashes(pipeline)

	switch g.SubjectScope {
	case SubjectScopeTeam:
		return team
	default:
		// default to SubjectScopePipeline
		fallthrough
	case SubjectScopePipeline:
		return fmt.Sprintf("%s/%s", team, pipeline)
	}
}

func escapeSlashes(input string) string {
	return strings.ReplaceAll(input, "/", "%2F")
}

func GenerateNewRSAKey() (*jose.JSONWebKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}

	return &jose.JSONWebKey{
		KeyID:     generateRandomNumericString(),
		Algorithm: "RS256",
		Key:       privateKey,
		Use:       "sign",
	}, nil
}

func generateRandomNumericString() string {
	num, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		// should never happen
		panic(err)
	}
	return strconv.Itoa(int(num.Int64()))
}
