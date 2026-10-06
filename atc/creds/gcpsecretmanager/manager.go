package gcpsecretmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc/creds"
	"google.golang.org/api/option"
)

const (
	// Segments are delimited by DefaultSegmentDelimiter. Shared secrets get their own
	// root so they can never collide with a team- or pipeline-scoped ID.
	DefaultPipelineSecretTemplate = "concourse--{{.Team}}--{{.Pipeline}}--{{.Secret}}"
	DefaultTeamSecretTemplate     = "concourse--{{.Team}}--{{.Secret}}"
	DefaultSharedSecretTemplate   = "concourse-shared--{{.Secret}}"

	DefaultRequestTimeout = 10 * time.Second

	healthCheckSecretID = "__concourse-health-check"
)

// Either a project ID (6-30 chars) or a numeric project number.
var projectPattern = regexp.MustCompile(`^([a-z][a-z0-9-]{4,28}[a-z0-9]|[0-9]+)$`)

type Manager struct {
	ProjectID string `mapstructure:"project" long:"project" description:"GCP project ID containing the secrets"`

	// When neither is set, Application Default Credentials are used.
	CredentialsFile string `mapstructure:"credentials_file" long:"credentials-file" description:"Path to a GCP service account JSON key file. Leave unset to use Application Default Credentials / Workload Identity."`
	CredentialsJSON string `mapstructure:"credentials_json" long:"credentials-json" description:"Inline GCP service account JSON key. Leave unset to use Application Default Credentials / Workload Identity."`

	RequestTimeout time.Duration `mapstructure:"request_timeout" long:"request-timeout" default:"10s" description:"Timeout applied to each Secret Manager API request"`

	SegmentDelimiter       string `mapstructure:"segment_delimiter" long:"segment-delimiter" default:"--" description:"Separator between the segments of a secret ID template, made of hyphens and underscores. Team, pipeline and var names may not contain it or begin or end with any of its characters."`
	PipelineSecretTemplate string `mapstructure:"pipeline_secret_template" long:"pipeline-secret-template" default:"concourse--{{.Team}}--{{.Pipeline}}--{{.Secret}}" description:"Google Secret Manager secret ID template used for pipeline specific parameter"`
	TeamSecretTemplate     string `mapstructure:"team_secret_template" long:"team-secret-template" default:"concourse--{{.Team}}--{{.Secret}}" description:"Google Secret Manager secret ID template used for team specific parameter"`
	SharedSecretTemplate   string `mapstructure:"shared_secret_template" long:"shared-secret-template" default:"concourse-shared--{{.Secret}}" description:"Google Secret Manager secret ID template used for shared parameter that can be used by all teams and pipelines"`

	SecretManager *SecretManager
}

func (manager *Manager) Init(log lager.Logger) error {
	client, err := manager.newClient(context.Background())
	if err != nil {
		log.Error("create-gcp-secretmanager-client", err)
		return err
	}

	manager.SecretManager = NewSecretManager(
		log,
		client,
		manager.ProjectID,
		manager.requestTimeoutOrDefault(),
		nil,
		manager.segmentDelimiterOrDefault(),
	)

	return nil
}

func (manager *Manager) Health() (*creds.HealthResponse, error) {
	health := &creds.HealthResponse{
		Method: "AccessSecretVersion",
	}

	if manager.SecretManager == nil {
		health.Error = "credential manager is not initialized"
		return health, nil
	}

	_, _, _, err := manager.SecretManager.getSecretByID(healthCheckSecretID)
	if err != nil {
		health.Error = err.Error()
		return health, nil
	}

	health.Response = map[string]string{
		"status": "UP",
	}

	return health, nil
}

// MarshalJSON feeds /api/v1/info/creds; it never serializes credentials.
func (manager *Manager) MarshalJSON() ([]byte, error) {
	health, err := manager.Health()
	if err != nil {
		return nil, err
	}

	return json.Marshal(&map[string]any{
		"project":                  manager.ProjectID,
		"segment_delimiter":        manager.segmentDelimiterOrDefault(),
		"pipeline_secret_template": manager.PipelineSecretTemplate,
		"team_secret_template":     manager.TeamSecretTemplate,
		"shared_secret_template":   manager.SharedSecretTemplate,
		"health":                   health,
	})
}

func (manager *Manager) IsConfigured() bool {
	return manager.ProjectID != ""
}

func (manager *Manager) Validate() error {
	if !projectPattern.MatchString(manager.ProjectID) {
		return fmt.Errorf("invalid GCP project %q: must be a valid project ID or project number", manager.ProjectID)
	}

	if manager.RequestTimeout < 0 {
		return errors.New("request timeout must not be negative")
	}

	if manager.CredentialsFile != "" && manager.CredentialsJSON != "" {
		return errors.New("must provide only one of credentials file or credentials json")
	}

	_, err := manager.secretTemplates()
	return err
}

func (manager *Manager) NewSecretsFactory(log lager.Logger) (creds.SecretsFactory, error) {
	if manager.SecretManager == nil {
		return nil, errors.New("Credential manager is not initialized")
	}

	// Validated again here because var_source configs reach this
	// point at runtime without passing through Validate.
	templates, err := manager.secretTemplates()
	if err != nil {
		return nil, err
	}

	// Reuse the client from Init: one gRPC connection per manager, closed by Close.
	return NewSecretManagerFactory(
		log,
		manager.SecretManager.api,
		manager.ProjectID,
		manager.requestTimeoutOrDefault(),
		templates,
		manager.segmentDelimiterOrDefault(),
	), nil
}

// secretTemplates builds the pipeline, team and shared templates, in lookup
// order. It rejects any template that can render an illegal secret ID, does
// not delimit its placeholders, or can render an ID that another template
// also renders.
func (manager *Manager) secretTemplates() ([]*creds.SecretTemplate, error) {
	delimiter := manager.segmentDelimiterOrDefault()
	if err := validateDelimiter(delimiter); err != nil {
		return nil, err
	}

	configured := []struct{ name, template string }{
		{"pipeline-secret-template", manager.PipelineSecretTemplate},
		{"team-secret-template", manager.TeamSecretTemplate},
		{"shared-secret-template", manager.SharedSecretTemplate},
	}

	templates := make([]*creds.SecretTemplate, 0, len(configured))
	tokens := make([][]templateToken, 0, len(configured))
	for _, c := range configured {
		built, err := creds.BuildSecretTemplate(c.name, c.template)
		if err != nil {
			return nil, err
		}

		sample, err := validateTemplate(built)
		if err != nil {
			return nil, err
		}
		if !secretIDPattern.MatchString(sample) {
			return nil, fmt.Errorf("%s produces an invalid Google Secret Manager secret ID (%q): only letters, numerals, hyphens and underscores are permitted", c.name, sample)
		}

		templateTokens, err := tokenizeTemplate(c.name, built, delimiter)
		if err != nil {
			return nil, err
		}

		for i, other := range tokens {
			if tokensOverlap(templateTokens, other) {
				return nil, fmt.Errorf("%s and %s can produce the same secret ID: give each scope a distinct literal prefix or a different number of segments", configured[i].name, c.name)
			}
		}

		templates = append(templates, built)
		tokens = append(tokens, templateTokens)
	}

	return templates, nil
}

func (manager *Manager) Close(logger lager.Logger) {
	if manager.SecretManager == nil || manager.SecretManager.api == nil {
		return
	}

	if err := manager.SecretManager.api.Close(); err != nil {
		logger.Error("Failed to close GCP Secret Manager client", err)
	}
}

func (manager *Manager) newClient(ctx context.Context) (SecretManagerAPI, error) {
	var opts []option.ClientOption

	switch {
	case manager.CredentialsJSON != "":
		opts = append(opts, option.WithCredentialsJSON([]byte(manager.CredentialsJSON)))
	case manager.CredentialsFile != "":
		opts = append(opts, option.WithCredentialsFile(manager.CredentialsFile))
	}

	client, err := secretmanager.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// validateTemplate expands a template with placeholders so its literal parts
// can be checked against the secret ID character set.
func validateTemplate(tmpl *creds.SecretTemplate) (string, error) {
	var buf bytes.Buffer
	err := tmpl.Execute(&buf, struct {
		Team     string
		Pipeline string
		Secret   string
	}{"team", "pipeline", "secret"})
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (manager *Manager) segmentDelimiterOrDefault() string {
	if manager.SegmentDelimiter == "" {
		return DefaultSegmentDelimiter
	}
	return manager.SegmentDelimiter
}

func (manager *Manager) requestTimeoutOrDefault() time.Duration {
	if manager.RequestTimeout <= 0 {
		return DefaultRequestTimeout
	}
	return manager.RequestTimeout
}
