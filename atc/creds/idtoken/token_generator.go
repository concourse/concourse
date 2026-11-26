package idtoken

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/concourse/concourse/v8/atc/creds"
	"github.com/concourse/concourse/v8/atc/db"
	"github.com/concourse/concourse/v8/vars"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
)

type SubjectScope string

const (
	SubjectScopeEmpty    SubjectScope = ""
	SubjectScopeTeam     SubjectScope = "team"
	SubjectScopePipeline SubjectScope = "pipeline"
	SubjectScopeInstance SubjectScope = "instance"
	SubjectScopeJob      SubjectScope = "job"

	DefaultAlgorithm = jose.RS256

	// AWSSessionTagsClaim is the claim AWS STS reads session tags from during
	// AssumeRoleWithWebIdentity.
	// See https://docs.aws.amazon.com/IAM/latest/UserGuide/id_session-tags.html#id_session-tags_adding-assume-role-idp
	AWSSessionTagsClaim = "https://aws.amazon.com/tags"

	awsInstanceVarTagPrefix = "instance_var:"

	// See https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_iam-quotas.html#reference_iam-quotas-entity-length
	awsSessionTagsMax           = 50
	awsSessionTagKeyMaxLength   = 128
	awsSessionTagValueMaxLength = 256
)

// characters AWS allows in session tag keys and values
var awsSessionTagCharacters = regexp.MustCompile(`^[\p{L}\p{Z}\p{N}_.:/=+\-@]*$`)

type awsTagsClaim struct {
	PrincipalTags map[string][]string `json:"principal_tags"`
}

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

	// AWSSessionTags adds team, pipeline, job and instance vars as AWS
	// session tags
	AWSSessionTags bool
}

func (g TokenGenerator) GenerateToken(params creds.SecretLookupParams) (token string, validUntil time.Time, err error) {
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

	jti, err := uuid.NewRandom()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("error generating UUID: %w", err)
	}

	claims := jwt.Claims{
		ID:       jti.String(),
		Issuer:   g.Issuer,
		IssuedAt: jwt.NewNumericDate(now),
		Audience: jwt.Audience(g.Audience),
		Subject:  generateSubject(g.SubjectScope, params),
		Expiry:   jwt.NewNumericDate(validUntil),
	}

	customClaims := struct {
		Team         string         `json:"team"`
		Pipeline     string         `json:"pipeline"`
		Job          string         `json:"job"`
		InstanceVars map[string]any `json:"instance_vars,omitempty"`
	}{
		Team:         params.Team,
		Pipeline:     params.Pipeline,
		Job:          params.Job,
		InstanceVars: params.InstanceVars,
	}

	builder := jwt.Signed(signer).Claims(claims).Claims(customClaims)

	if g.AWSSessionTags {
		tags, err := generateAWSSessionTags(params)
		if err != nil {
			return "", time.Time{}, err
		}
		builder = builder.Claims(map[string]any{AWSSessionTagsClaim: tags})
	}

	signed, err := builder.Serialize()
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

func generateSubject(scope SubjectScope, params creds.SecretLookupParams) string {
	team := escapeSlashes(params.Team)
	pipeline := escapeSlashes(params.Pipeline)
	ivars := escapeSlashes(params.InstanceVars.String())
	job := escapeSlashes(params.Job)

	switch scope {
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

// generateAWSSessionTags fails instead of sanitizing invalid keys or values,
// as rewriting them could make two pipelines end up with the same tags
func generateAWSSessionTags(params creds.SecretLookupParams) (awsTagsClaim, error) {
	type tag struct{ key, value string }

	tags := []tag{
		{"team", params.Team},
		{"pipeline", params.Pipeline},
	}
	// job is empty outside of a job (e.g. resource checks)
	if params.Job != "" {
		tags = append(tags, tag{"job", params.Job})
	}

	// AWS only accepts a single string per tag, so every (nested) instance
	// var becomes its own tag, e.g. {env: prod, app: {name: x}} becomes
	// instance_var:env=prod and instance_var:app.name=x
	ivars := vars.StaticVariables(params.InstanceVars).Flatten()
	slices.SortFunc(ivars, func(a, b vars.KVPair) int {
		return strings.Compare(instanceVarTagKey(a.Ref), instanceVarTagKey(b.Ref))
	})
	for _, kv := range ivars {
		value, err := instanceVarTagValue(kv.Value)
		if err != nil {
			return awsTagsClaim{}, fmt.Errorf("cannot use instance var %q as aws session tag: %w", kv.Ref.String(), err)
		}
		tags = append(tags, tag{instanceVarTagKey(kv.Ref), value})
	}

	if len(tags) > awsSessionTagsMax {
		return awsTagsClaim{}, fmt.Errorf("cannot use more than %d aws session tags, got %d", awsSessionTagsMax, len(tags))
	}

	principalTags := make(map[string][]string, len(tags))
	// AWS treats tag keys case-insensitively, so e.g. instance vars Env and
	// env (or a.b and {a: {b: ...}}) would silently override each other
	seenKeys := make(map[string]string, len(tags))
	for _, t := range tags {
		if err := validateAWSSessionTag(t.key, t.value); err != nil {
			return awsTagsClaim{}, fmt.Errorf("cannot use %s %q as aws session tag: %w", t.key, t.value, err)
		}
		if other, found := seenKeys[strings.ToLower(t.key)]; found {
			return awsTagsClaim{}, fmt.Errorf("conflicting aws session tags %q and %q", other, t.key)
		}
		seenKeys[strings.ToLower(t.key)] = t.key
		principalTags[t.key] = []string{t.value}
	}

	return awsTagsClaim{PrincipalTags: principalTags}, nil
}

func instanceVarTagKey(ref vars.Reference) string {
	return awsInstanceVarTagPrefix + strings.Join(append([]string{ref.Path}, ref.Fields...), ".")
}

func instanceVarTagValue(value any) (string, error) {
	if str, ok := value.(string); ok {
		return str, nil
	}
	// numbers, booleans and null use their JSON form, e.g. 1, true
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func validateAWSSessionTag(key, value string) error {
	if utf8.RuneCountInString(key) > awsSessionTagKeyMaxLength {
		return fmt.Errorf("key longer than %d characters", awsSessionTagKeyMaxLength)
	}
	if utf8.RuneCountInString(value) > awsSessionTagValueMaxLength {
		return fmt.Errorf("value longer than %d characters", awsSessionTagValueMaxLength)
	}
	if !awsSessionTagCharacters.MatchString(key) || !awsSessionTagCharacters.MatchString(value) {
		return fmt.Errorf("only letters, numbers, spaces and _ . : / = + - @ are allowed")
	}
	if strings.HasPrefix(strings.ToLower(key), "aws:") || strings.HasPrefix(strings.ToLower(value), "aws:") {
		return fmt.Errorf("the aws: prefix is reserved")
	}
	return nil
}

func escapeSlashes(input string) string {
	return strings.ReplaceAll(input, "/", "%2F")
}
