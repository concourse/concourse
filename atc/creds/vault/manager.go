package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"time"

	"code.cloudfoundry.org/lager/v3"

	"github.com/concourse/concourse/atc/creds"
	"github.com/go-viper/mapstructure/v2"
)

type VaultManager struct {
	URL string `mapstructure:"url" long:"url" description:"Vault server address used to access secrets."`

	PathPrefix      string        `mapstructure:"path_prefix" long:"path-prefix" default:"/concourse" description:"Path under which to namespace credential lookup."`
	PathPrefixes    []string      `mapstructure:"path_prefixes" long:"path-prefixes" description:"Multiple paths under which to namespace credential lookup."`
	LookupTemplates []string      `mapstructure:"lookup_templates" long:"lookup-templates" default:"/{{.Team}}/{{.Pipeline}}/{{.Secret}}" default:"/{{.Team}}/{{.Secret}}" description:"Path templates for credential lookup"`
	SharedPath      string        `mapstructure:"shared_path" long:"shared-path" description:"Path under which to lookup shared credentials."`
	Namespace       string        `mapstructure:"namespace" long:"namespace"   description:"Vault namespace to use for authentication and secret lookup."`
	LoginTimeout    time.Duration `mapstructure:"login_timeout" long:"login-timeout" default:"60s" description:"Timeout value for Vault login."`
	QueryTimeout    time.Duration `mapstructure:"query_timeout" long:"query-timeout" default:"60s" description:"Timeout value for Vault query."`

	ClientConfig ClientConfig `mapstructure:",squash"`
	TLS          TLSConfig    `mapstructure:",squash"`
	Auth         AuthConfig   `mapstructure:",squash"`

	Client        *APIClient
	ReAuther      *ReAuther
	SecretFactory *vaultFactory
}

type ClientConfig struct {
	DisableSRVLookup bool `mapstructure:"disable_srv_lookup" long:"disable-srv-lookup" description:"When vault URL contains NO port, use this flag to control if the client will lookup the host through DNS SRV lookup. When vault URL contains port, SRV lookup is diabled and this flag has no impact."`
}

type TLSConfig struct {
	CACert     string `mapstructure:"ca_cert"`
	CACertFile string `long:"ca-cert"              description:"Path to a PEM-encoded CA cert file to use to verify the vault server SSL cert."`
	CAPath     string `long:"ca-path"              description:"Path to a directory of PEM-encoded CA cert files to verify the vault server SSL cert."`

	ClientCert     string `mapstructure:"client_cert"`
	ClientCertFile string `long:"client-cert"          description:"Path to the client certificate for Vault authorization."`

	ClientKey     string `mapstructure:"client_key"`
	ClientKeyFile string `long:"client-key"           description:"Path to the client private key for Vault authorization."`

	ServerName string `mapstructure:"server_name" long:"server-name"          description:"If set, is used to set the SNI host when connecting via TLS."`
	Insecure   bool   `mapstructure:"insecure_skip_verify" long:"insecure-skip-verify" description:"Enable insecure SSL verification."`
}

type AuthConfig struct {
	ClientToken     string `mapstructure:"client_token" long:"client-token" description:"Client token for accessing secrets within the Vault server."`
	ClientTokenPath string `mapstructure:"client_token_path" long:"client-token-path" description:"Absolute path to a file containing the Vault client token."`

	Backend       string        `mapstructure:"auth_backend" long:"auth-backend"               description:"Auth backend to use for logging in to Vault."`
	BackendMaxTTL time.Duration `mapstructure:"auth_backend_max_ttl" long:"auth-backend-max-ttl"       description:"Time after which to force a re-login. If not set, the token will just be continuously renewed."`
	RetryMax      time.Duration `mapstructure:"auth_retry_max" long:"retry-max"     default:"5m" description:"The maximum time between retries when logging in or re-authing a secret."`
	RetryInitial  time.Duration `mapstructure:"auth_retry_initial" long:"retry-initial" default:"1s" description:"The initial time between retries when logging in or re-authing a secret."`

	Params map[string]string `mapstructure:"auth_params" long:"auth-param"  description:"Paramter to pass when logging in via the backend. Can be specified multiple times." value-name:"NAME:VALUE"`
}

func (manager *VaultManager) Init(log lager.Logger) error {
	var err error

	manager.Client, err = NewAPIClient(log, manager.URL, manager.ClientConfig, manager.TLS, manager.Auth, manager.Namespace, manager.QueryTimeout)
	if err != nil {
		return err
	}

	return nil
}

func (manager *VaultManager) MarshalJSON() ([]byte, error) {
	health, err := manager.Health()
	if err != nil {
		return nil, err
	}

	return json.Marshal(&map[string]any{
		"url":                manager.URL,
		"path_prefix":        manager.PathPrefix,
		"path_prefixes":      manager.PathPrefixes,
		"lookup_templates":   manager.LookupTemplates,
		"shared_path":        manager.SharedPath,
		"namespace":          manager.Namespace,
		"ca_cert":            manager.TLS.CACert,
		"server_name":        manager.TLS.ServerName,
		"auth_backend":       manager.Auth.Backend,
		"auth_max_ttl":       manager.Auth.BackendMaxTTL,
		"auth_retry_max":     manager.Auth.RetryMax,
		"auth_retry_initial": manager.Auth.RetryInitial,
		"health":             health,
	})
}

func (manager *VaultManager) Config(config map[string]any) error {
	// apply defaults
	manager.PathPrefix = "/concourse"
	manager.PathPrefixes = []string{}
	manager.Auth.RetryMax = 5 * time.Minute
	manager.Auth.RetryInitial = time.Second
	manager.LoginTimeout = 60 * time.Second
	manager.QueryTimeout = 60 * time.Second

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook:  mapstructure.StringToTimeDurationHookFunc(),
		ErrorUnused: true,
		Result:      &manager,
	})
	if err != nil {
		return err
	}

	err = decoder.Decode(config)
	if err != nil {
		return err
	}

	// Fill in default templates if not otherwise set (done here so
	// that these are effective all together or not at all, rather
	// than combining the defaults with a user's custom setting)
	if _, setsTemplates := config["lookup_templates"]; !setsTemplates {
		manager.LookupTemplates = []string{
			"/{{.Team}}/{{.Pipeline}}/{{.Secret}}",
			"/{{.Team}}/{{.Secret}}",
		}
	}

	return nil
}

func (manager VaultManager) IsConfigured() bool {
	return manager.URL != ""
}

func (manager VaultManager) Validate() error {
	_, err := url.Parse(manager.URL)
	if err != nil {
		return fmt.Errorf("invalid URL: %s", err)
	}

	if manager.PathPrefix == "" && len(manager.PathPrefixes) == 0 {
		return fmt.Errorf("path prefix must be a non-empty string")
	}

	if manager.PathPrefix != "" && len(manager.PathPrefixes) > 0 {
		return fmt.Errorf("only path prefix or path prefixes may be set, but not both")
	}

	for _, prefix := range getPrefixes(manager.PathPrefixes, manager.PathPrefix) {
		for i, tmpl := range manager.LookupTemplates {
			name := fmt.Sprintf("lookup-template-%d", i)
			if _, err := creds.BuildSecretTemplate(name, prefix+tmpl); err != nil {
				return err
			}
		}
	}

	if manager.Auth.ClientToken != "" {
		return nil
	}

	if manager.Auth.ClientTokenPath != "" {
		return nil
	}

	if manager.Auth.Backend != "" {
		return nil
	}

	return errors.New("must configure client token or auth backend")
}

func (manager VaultManager) Health() (*creds.HealthResponse, error) {
	health := &creds.HealthResponse{
		Method: "/v1/sys/health",
	}

	response, err := manager.Client.health()
	if err != nil {
		health.Error = err.Error()
		return health, nil
	}

	health.Response = response
	return health, nil
}

func (manager *VaultManager) NewSecretsFactory(logger lager.Logger) (creds.SecretsFactory, error) {
	if manager.SecretFactory == nil {

		templates := []*creds.SecretTemplate{}
		for _, prefix := range getPrefixes(manager.PathPrefixes, manager.PathPrefix) {
			for i, tmpl := range manager.LookupTemplates {
				name := fmt.Sprintf("lookup-template-%d", i)
				scopedTemplate := path.Join(prefix, tmpl)
				if template, err := creds.BuildSecretTemplate(name, scopedTemplate); err != nil {
					return nil, err
				} else {
					templates = append(templates, template)
				}
			}
		}

		manager.ReAuther = NewReAuther(
			logger,
			manager.Client,
			manager.Auth.BackendMaxTTL,
			manager.Auth.RetryInitial,
			manager.Auth.RetryMax,
		)

		manager.SecretFactory = NewVaultFactory(
			manager.Client,
			manager.LoginTimeout,
			manager.ReAuther.LoggedIn(),
			manager.PathPrefix,
			manager.PathPrefixes,
			templates,
			manager.SharedPath,
		)
	}

	return manager.SecretFactory, nil
}

func (manager VaultManager) Close(logger lager.Logger) {
	manager.ReAuther.Close()
}

func getPrefixes(prefixes []string, prefix string) []string {
	if prefix != "" {
		prefixes = append(prefixes, prefix)
	}
	return prefixes
}
