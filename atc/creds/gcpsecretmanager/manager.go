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
	// Secret IDs allow no slashes, so a double hyphen delimits segments.
	DefaultPipelineSecretTemplate = "concourse--{{.Team}}--{{.Pipeline}}--{{.Secret}}"
	DefaultTeamSecretTemplate     = "concourse--{{.Team}}--{{.Secret}}"
	DefaultSharedSecretTemplate   = "concourse--{{.Secret}}"

	DefaultSecretVersion  = "latest"
	DefaultRequestTimeout = 10 * time.Second

	healthCheckSecretID = "__concourse-health-check"
)

// Either a project ID (6-30 chars) or a numeric project number.
var projectPattern = regexp.MustCompile(`^([a-z][a-z0-9-]{4,28}[a-z0-9]|[0-9]+)$`)

// The "latest" alias or a specific numeric version.
var versionPattern = regexp.MustCompile(`^(latest|[0-9]+)$`)

type Manager struct {
	ProjectID string `mapstructure:"project" long:"project" description:"GCP project ID containing the secrets"`

	// When neither is set, Application Default Credentials are used.
	CredentialsFile string `mapstructure:"credentials_file" long:"credentials-file" description:"Path to a GCP service account JSON key file. Leave unset to use Application Default Credentials / Workload Identity."`
	CredentialsJSON string `mapstructure:"credentials_json" long:"credentials-json" description:"Inline GCP service account JSON key. Leave unset to use Application Default Credentials / Workload Identity."`

	SecretVersion  string        `mapstructure:"secret_version" long:"secret-version" default:"latest" description:"Secret version to access; either 'latest' or a specific numeric version"`
	RequestTimeout time.Duration `mapstructure:"request_timeout" long:"request-timeout" default:"10s" description:"Timeout applied to each Secret Manager API request"`

	PipelineSecretTemplate string `mapstructure:"pipeline_secret_template" long:"pipeline-secret-template" default:"concourse--{{.Team}}--{{.Pipeline}}--{{.Secret}}" description:"Google Secret Manager secret ID template used for pipeline specific parameter"`
	TeamSecretTemplate     string `mapstructure:"team_secret_template" long:"team-secret-template" default:"concourse--{{.Team}}--{{.Secret}}" description:"Google Secret Manager secret ID template used for team specific parameter"`
	SharedSecretTemplate   string `mapstructure:"shared_secret_template" long:"shared-secret-template" default:"concourse--{{.Secret}}" description:"Google Secret Manager secret ID template used for shared parameter that can be used by all teams and pipelines"`

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
		manager.secretVersionOrDefault(),
		manager.requestTimeoutOrDefault(),
		nil,
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
		"secret_version":           manager.secretVersionOrDefault(),
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

	if !versionPattern.MatchString(manager.secretVersionOrDefault()) {
		return fmt.Errorf("invalid secret version %q: must be 'latest' or a numeric version", manager.SecretVersion)
	}

	if manager.RequestTimeout < 0 {
		return errors.New("request timeout must not be negative")
	}

	if manager.CredentialsFile != "" && manager.CredentialsJSON != "" {
		return errors.New("must provide only one of credentials file or credentials json")
	}

	templates := map[string]string{
		"pipeline-secret-template": manager.PipelineSecretTemplate,
		"team-secret-template":     manager.TeamSecretTemplate,
		"shared-secret-template":   manager.SharedSecretTemplate,
	}
	for name, tmpl := range templates {
		built, err := creds.BuildSecretTemplate(name, tmpl)
		if err != nil {
			return err
		}

		sample, err := validateTemplate(built)
		if err != nil {
			return err
		}
		if !secretIDPattern.MatchString(sample) {
			return fmt.Errorf("%s produces an invalid Google Secret Manager secret ID (%q): only letters, numerals, hyphens and underscores are permitted", name, sample)
		}
	}

	return nil
}

func (manager *Manager) NewSecretsFactory(log lager.Logger) (creds.SecretsFactory, error) {
	if manager.SecretManager == nil {
		return nil, errors.New("Credential manager is not initialized")
	}

	pipelineSecretTemplate, err := creds.BuildSecretTemplate("pipeline-secret-template", manager.PipelineSecretTemplate)
	if err != nil {
		return nil, err
	}

	teamSecretTemplate, err := creds.BuildSecretTemplate("team-secret-template", manager.TeamSecretTemplate)
	if err != nil {
		return nil, err
	}

	sharedSecretTemplate, err := creds.BuildSecretTemplate("shared-secret-template", manager.SharedSecretTemplate)
	if err != nil {
		return nil, err
	}

	// Reuse the client from Init: one gRPC connection per manager, closed by Close.
	return NewSecretManagerFactory(
		log,
		manager.SecretManager.api,
		manager.ProjectID,
		manager.secretVersionOrDefault(),
		manager.requestTimeoutOrDefault(),
		[]*creds.SecretTemplate{pipelineSecretTemplate, teamSecretTemplate, sharedSecretTemplate},
	), nil
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

func (manager *Manager) secretVersionOrDefault() string {
	if manager.SecretVersion == "" {
		return DefaultSecretVersion
	}
	return manager.SecretVersion
}

func (manager *Manager) requestTimeoutOrDefault() time.Duration {
	if manager.RequestTimeout <= 0 {
		return DefaultRequestTimeout
	}
	return manager.RequestTimeout
}
