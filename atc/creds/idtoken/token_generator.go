package idtoken

import (
	"crypto/rand"
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

	DefaultAlgorithm = jose.RS256
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

	SubjectScope SubjectScope
	Audience     []string
	ExpiresIn    time.Duration
	Algorithm    jose.SignatureAlgorithm
}

func (g TokenGenerator) GenerateToken(team, pipeline string) (token string, validUntil time.Time, err error) {
	now := time.Now()
	validUntil = now.Add(g.ExpiresIn)

	signingKey, err := g.getSigningKey()
	if err != nil {
		return "", time.Time{}, err
	}

	signer, err := jose.NewSigner(*signingKey, &jose.SignerOptions{})
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

func (g TokenGenerator) getSigningKey() (*jose.SigningKey, error) {
	alg := g.Algorithm
	if alg == "" {
		alg = DefaultAlgorithm
	}

	var keyType db.SigningKeyType
	if strings.HasPrefix(string(alg), "RS") {
		keyType = db.SigningKeyTypeRSA
	} else if strings.HasPrefix(string(alg), "ES") {
		keyType = db.SigningKeyTypeEC
	} else {
		return nil, fmt.Errorf("unsupported signing algorithm")
	}

	latestKey, err := g.SigningKeyFactory.GetNewestKey(keyType)
	if err != nil {
		return nil, fmt.Errorf("failed to get a signing key for the selected algorithm %w", err)
	}
	return &jose.SigningKey{
		Algorithm: alg,
		Key:       latestKey.JWK(),
	}, nil
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

func generateRandomNumericString() string {
	num, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		// should never happen
		panic(err)
	}
	return strconv.Itoa(int(num.Int64()))
}
