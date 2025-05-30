package idtoken

import (
	"fmt"
	"strings"
	"time"

	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"

	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
)

type SubjectScope string

const (
	SubjectScopeEmpty    SubjectScope = ""
	SubjectScopeTeam     SubjectScope = "team"
	SubjectScopePipeline SubjectScope = "pipeline"
	SubjectScopeInstance SubjectScope = "instance"
	SubjectScopeJob      SubjectScope = "job"

	DefaultAlgorithm = jose.RS256
)

func (s SubjectScope) Valid() bool {
	switch s {
	case SubjectScopeEmpty, SubjectScopeTeam, SubjectScopePipeline, SubjectScopeInstance, SubjectScopeJob:
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

func (g TokenGenerator) GenerateToken(context creds.SecretLookupContext) (token string, validUntil time.Time, err error) {
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
		Subject:  g.generateSubject(context),
		Expiry:   jwt.NewNumericDate(validUntil),
	}

	customClaims := struct {
		Team         string         `json:"team"`
		Pipeline     string         `json:"pipeline"`
		Job          string         `json:"job"`
		InstanceVars map[string]any `json:"instance_vars,omitempty"`
	}{
		Team:         context.Team,
		Pipeline:     context.Pipeline,
		Job:          context.Job,
		InstanceVars: context.InstanceVars,
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

func (g TokenGenerator) generateSubject(context creds.SecretLookupContext) string {
	team := escapeSlashes(context.Team)
	pipeline := escapeSlashes(context.Pipeline)
	ivars := escapeSlashes(context.InstanceVars.String())
	job := escapeSlashes(context.Job)

	switch g.SubjectScope {
	case SubjectScopeTeam:
		return team
	default:
		// default to SubjectScopePipeline
		fallthrough
	case SubjectScopePipeline:
		return fmt.Sprintf("%s/%s", team, pipeline)
	case SubjectScopeInstance:
		return fmt.Sprintf("%s/%s/%s", team, pipeline, ivars)
	case SubjectScopeJob:
		return fmt.Sprintf("%s/%s/%s/%s", team, pipeline, ivars, job)
	}
}

func escapeSlashes(input string) string {
	return strings.ReplaceAll(input, "/", "%2F")
}
