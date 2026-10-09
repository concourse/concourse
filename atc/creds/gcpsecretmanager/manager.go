package gcpsecretmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/credentials"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc/creds"
	"google.golang.org/api/option"
)

const (
	// Shared secrets get their own root, so they can never collide with a
	// team- or pipeline-scoped ID.
	DefaultPipelineSecretTemplate = "/concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}"
	DefaultTeamSecretTemplate     = "/concourse/{{.Team}}/{{.Secret}}"
	DefaultSharedSecretTemplate   = "/concourse-shared/{{.Secret}}"

	DefaultRequestTimeout = 10 * time.Second

	healthCheckSecretID = "__concourse-health-check"
)

// Either a project ID (6-30 chars) or a numeric project number.
var projectPattern = regexp.MustCompile(`^([a-z][a-z0-9-]{4,28}[a-z0-9]|[0-9]+)$`)

type Manager struct {
	ProjectID string `mapstructure:"project" long:"project" description:"GCP project ID containing the secrets"`

	// When neither is set, Application Default Credentials are used.
	CredentialsFile string `mapstructure:"credentials_file" long:"credentials-file" description:"Path to a GCP service account JSON key file. Not available in a var_source. Leave unset to use Application Default Credentials / Workload Identity."`
	CredentialsJSON string `mapstructure:"credentials_json" long:"credentials-json" description:"Inline GCP service account JSON key. Leave unset to use Application Default Credentials / Workload Identity."`

	RequestTimeout time.Duration `mapstructure:"request_timeout" long:"request-timeout" default:"10s" description:"Timeout applied to each Secret Manager API request"`

	PipelineSecretTemplate string `mapstructure:"pipeline_secret_template" long:"pipeline-secret-template" default:"/concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}" description:"Secret path template used for pipeline specific parameter. Mapped to a Google Secret Manager secret ID by dropping the leading slash and replacing each remaining slash with '--'."`
	TeamSecretTemplate     string `mapstructure:"team_secret_template" long:"team-secret-template" default:"/concourse/{{.Team}}/{{.Secret}}" description:"Secret path template used for team specific parameter. Mapped to a Google Secret Manager secret ID by dropping the leading slash and replacing each remaining slash with '--'."`
	SharedSecretTemplate   string `mapstructure:"shared_secret_template" long:"shared-secret-template" default:"/concourse-shared/{{.Secret}}" description:"Secret path template used for shared parameter that can be used by all teams and pipelines, under a root of its own. Mapped to a Google Secret Manager secret ID by dropping the leading slash and replacing each remaining slash with '--'."`

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
	), nil
}

// secretTemplates builds the pipeline, team and shared templates, in lookup
// order. It rejects any template that can render an illegal secret ID, does
// not give each placeholder its own segment, or can render an ID that another
// template also renders. The shared template must have its own root.
func (manager *Manager) secretTemplates() ([]*creds.SecretTemplate, error) {
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
		sample = secretPathToID(sample)
		if !secretIDPattern.MatchString(sample) {
			return nil, fmt.Errorf("%s produces an invalid Google Secret Manager secret ID (%q): only letters, numerals, hyphens and underscores are permitted", c.name, sample)
		}

		templateTokens, err := tokenizeTemplate(c.name, built)
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

	shared := len(configured) - 1
	for i := range shared {
		if tokens[i][0].literal == tokens[shared][0].literal {
			return nil, fmt.Errorf("%s and %s both begin with %q: shared secrets need their own root, such as /concourse-shared/{{.Secret}}", configured[i].name, configured[shared].name, tokens[shared][0].literal)
		}
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

	authCreds, err := manager.authCredentials()
	if err != nil {
		return nil, err
	}
	if authCreds != nil {
		opts = append(opts, option.WithAuthCredentials(authCreds))
	}

	client, err := secretmanager.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// authCredentials loads the configured service account key, or returns nil
// so the client falls back to Application Default Credentials.
//
// Only service account keys are accepted. option.WithCredentialsJSON and
// option.WithCredentialsFile accept any credential type, including
// external_account configurations that make the client read local files, call
// arbitrary URLs or run executables. That matters because pipeline authors
// control var_source configs. The typed replacements (WithAuthCredentialsJSON
// and WithAuthCredentialsFile) don't help: the gRPC transport drops the type
// before loading the credentials.
//
// Self-signed JWTs, which the generated client uses by default, mean the
// token_uri inside the key is never contacted.
func (manager *Manager) authCredentials() (*auth.Credentials, error) {
	var keyJSON []byte
	switch {
	case manager.CredentialsJSON != "":
		keyJSON = []byte(manager.CredentialsJSON)
	case manager.CredentialsFile != "":
		data, err := os.ReadFile(manager.CredentialsFile)
		if err != nil {
			return nil, fmt.Errorf("read GCP credentials file: %w", err)
		}
		keyJSON = data
	default:
		return nil, nil
	}

	authCreds, err := credentials.NewCredentialsFromJSON(credentials.ServiceAccount, keyJSON, &credentials.DetectOptions{
		Scopes:           secretmanager.DefaultAuthScopes(),
		UseSelfSignedJWT: true,
	})
	if err != nil {
		return nil, fmt.Errorf("load GCP service account credentials: %w", err)
	}

	return authCreds, nil
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

func (manager *Manager) requestTimeoutOrDefault() time.Duration {
	if manager.RequestTimeout <= 0 {
		return DefaultRequestTimeout
	}
	return manager.RequestTimeout
}
