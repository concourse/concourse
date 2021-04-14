package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"os"

	"github.com/clarafu/envstruct"
	concourseCmd "github.com/concourse/concourse/cmd"
	v "github.com/concourse/concourse/cmd/concourse/validator"
	"github.com/concourse/concourse/tsa/tsacmd"
	"github.com/concourse/flag"
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/concourse/concourse/atc/atccmd"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
)

var webCmd WebConfig

var WebCommand = &cobra.Command{
	Use:   "web",
	Short: "Run the web UI and build scheduler",
	Long: `Concourse relies on the web component to start up the ATC
	and the TSA.`,
	RunE: InitializeWeb,
}

func init() {
	WebCommand.Flags().Var(&webCmd.ConfigFile, "config", "config file (default is $HOME/.cobra.yaml)")

	WebCommand.Flags().StringVar(&webCmd.PeerAddress, "peer-address", "127.0.0.1", "Network address of this web node, reachable by other web nodes. Used for forwarded worker addresses.")

	webCmd.RunConfig = atccmd.CmdDefaults
	webCmd.TSAConfig = tsacmd.CmdDefaults

	// IMPORTANT!: Can be removed when flags no longer supported
	atccmd.InitializeATCFlagsDEPRECATED(WebCommand, &webCmd.RunConfig)
	tsacmd.InitializeTSAFlagsDEPRECATED(WebCommand, &webCmd.TSAConfig)

	// TODO: Mark all flags as deprecated
}

type WebConfig struct {
	ConfigFile flag.File `env:"CONCOURSE_CONFIG_FILE"`

	PeerAddress string `yaml:"peer_address"`

	atccmd.RunConfig `yaml:"web" ignore_env:"true"`
	tsacmd.TSAConfig `yaml:"worker_gateway"`
}

func InitializeWeb(cmd *cobra.Command, args []string) error {
	// IMPORTANT! This can be removed after we completely deprecate flags
	fixupFlagDefaults(cmd, &webCmd)

	// Fetch out env values
	env := envstruct.Envstruct{
		Prefix:        "CONCOURSE",
		TagName:       "yaml",
		OverrideName:  "env",
		IgnoreTagName: "ignore_env",
		StripValue:    true,

		Parser: envstruct.Parser{
			Delimiter:   ",",
			Unmarshaler: yaml.Unmarshal,
		},
	}

	err := env.FetchEnv(&webCmd)
	if err != nil {
		return fmt.Errorf("fetch env: %s", err)
	}

	err = populateConfigFields(&webCmd)
	if err != nil {
		return fmt.Errorf("convert flags: %s", err)
	}

	// Fetch out the values set from the config file and overwrite the flag
	// values
	if webCmd.ConfigFile != "" {
		file, err := os.Open(string(webCmd.ConfigFile))
		if err != nil {
			return fmt.Errorf("open file: %s", err)
		}

		decoder := yaml.NewDecoder(file)
		err = decoder.Decode(&webCmd)
		if err != nil {
			return fmt.Errorf("decode config: %s", err)
		}
	}

	// Validate the values passed in by the user
	en := en.New()
	uni := ut.New(en, en)
	trans, _ := uni.GetTranslator("en")

	webValidator := v.NewValidator(trans)

	err = webValidator.Struct(webCmd)
	if err != nil {
		validationErrors := err.(validator.ValidationErrors)

		var errs *multierror.Error
		for _, validationErr := range validationErrors {
			errs = multierror.Append(
				errs,
				errors.New(validationErr.Translate(trans)),
			)
		}

		return errs.ErrorOrNil()
	}

	err = webCmd.Execute(cmd, args)
	if err != nil {
		return fmt.Errorf("failed to execute web: %s", err)
	}

	return nil
}

func (w *WebConfig) Execute(cmd *cobra.Command, args []string) error {
	runner, err := w.Runner(args)
	if err != nil {
		return err
	}

	return <-ifrit.Invoke(sigmon.New(runner)).Wait()
}

func (w *WebConfig) Runner(args []string) (ifrit.Runner, error) {
	if w.RunConfig.CLIArtifactsDir == "" {
		w.RunConfig.CLIArtifactsDir = flag.Dir(concourseCmd.DiscoverAsset("fly-assets"))
	}

	err := w.populateSharedFlags()
	if err != nil {
		return nil, err
	}

	atcRunner, err := w.RunConfig.Runner(args)
	if err != nil {
		return nil, err
	}

	tsaRunner, err := w.TSAConfig.Runner(args)
	if err != nil {
		return nil, err
	}

	logger, _ := w.RunConfig.Logger.Logger("web")
	return grouper.NewParallel(os.Interrupt, grouper.Members{
		{
			Name:   "atc",
			Runner: concourseCmd.NewLoggingRunner(logger.Session("atc-runner"), atcRunner),
		},
		{
			Name:   "tsa",
			Runner: concourseCmd.NewLoggingRunner(logger.Session("tsa-runner"), tsaRunner),
		},
	}), nil
}

func (w *WebConfig) populateSharedFlags() error {
	var signingKey *rsa.PrivateKey
	if w.RunConfig.Auth.AuthFlags.SigningKey == nil || w.RunConfig.Auth.AuthFlags.SigningKey.PrivateKey == nil {
		var err error
		signingKey, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return fmt.Errorf("failed to generate session signing key: %s", err)
		}

		w.RunConfig.Auth.AuthFlags.SigningKey = &flag.PrivateKey{PrivateKey: signingKey}
	} else {
		signingKey = w.RunConfig.Auth.AuthFlags.SigningKey.PrivateKey
	}

	w.TSAConfig.PeerAddress = w.PeerAddress

	if len(w.TSAConfig.ATCURLs) == 0 {
		w.TSAConfig.ATCURLs = flag.URLs{w.RunConfig.DefaultURL()}
	}

	if w.TSAConfig.TokenURL.URL == nil {
		tokenPath, _ := url.Parse("/sky/issuer/token")
		w.TSAConfig.TokenURL.URL = w.RunConfig.DefaultURL().URL.ResolveReference(tokenPath)
	}

	if w.TSAConfig.ClientSecret == "" {
		w.TSAConfig.ClientSecret = derivedCredential(signingKey, w.TSAConfig.ClientID)
	}

	if w.RunConfig.Server.ClientSecret == "" {
		w.RunConfig.Server.ClientSecret = derivedCredential(signingKey, w.RunConfig.Server.ClientID)
	}

	if w.RunConfig.Auth.AuthFlags.Clients == nil {
		w.RunConfig.Auth.AuthFlags.Clients = make(map[string]string)
	}

	w.RunConfig.Auth.AuthFlags.Clients[w.TSAConfig.ClientID] = w.TSAConfig.ClientSecret
	w.RunConfig.Auth.AuthFlags.Clients[w.RunConfig.Server.ClientID] = w.RunConfig.Server.ClientSecret

	// if we're using the 'aud' as the SystemClaimKey then we want to validate
	// that the SystemClaimValues contains our TSA Client. If it's not 'aud' then
	// we can't validate anything
	if w.RunConfig.SystemClaim.Key == "aud" {

		// if we're using the default SystemClaimValues then override these values
		// to make sure they include the TSA ClientID
		if len(w.RunConfig.SystemClaim.Values) == 1 {
			if w.RunConfig.SystemClaim.Values[0] == "concourse-worker" {
				w.RunConfig.SystemClaim.Values = []string{w.TSAConfig.ClientID}
			}
		}

		if err := w.validateSystemClaimValues(); err != nil {
			return err
		}
	}

	w.TSAConfig.ClusterName = w.RunConfig.Server.ClusterName
	w.TSAConfig.LogClusterName = w.RunConfig.Log.ClusterName

	return nil
}

func (w *WebConfig) validateSystemClaimValues() error {
	found := false
	for _, val := range w.RunConfig.SystemClaim.Values {
		if val == w.TSAConfig.ClientID {
			found = true
		}
	}

	if !found {
		return errors.New("at least one systemClaimValue must be equal to tsa-client-id")
	}

	return nil
}

func derivedCredential(key *rsa.PrivateKey, clientID string) string {
	return fmt.Sprintf("%x", sha256.Sum256(key.N.Append([]byte(clientID), 10)))
}

// DEPRECATED! This is used to set flag values to nil because flags will always
// populate the configuration fields with the default value and these fields
// have a default value of 0
func fixupFlagDefaults(cmd *cobra.Command, web *WebConfig) {
	// XXX: TEST THIS
	if !cmd.Flags().Changed("default-task-cpu-limit") {
		web.DefaultCpuLimit = nil
	}

	if !cmd.Flags().Changed("default-task-memory-limit") {
		web.DefaultMemoryLimit = nil
	}
}

// DEPRECATED! This is only used for converting integrations configured through
// flags/env into proper configuration from a config file. For example, the way
// to configure a metrics emitter with flags/env is through setting specific
// fields from the emitter but with the config file it is through setting
// "emitter:<value>". This helper method converts the integrations configured
// through flags/env into an understandable format.
func populateConfigFields(web *WebConfig) error {
	err := convertCredentialManagerFlags(web)
	if err != nil {
		return err
	}

	err = convertMetricEmitterFlags(web)
	if err != nil {
		return err
	}

	err = convertTracingProviderFlags(web)
	if err != nil {
		return err
	}

	convertAuthProviderFlags(web)

	return nil
}

func convertCredentialManagerFlags(web *WebConfig) error {
	var configuredCredentialManagers []string

	c := web.CredentialManagers
	if c.Conjur.ConjurApplianceUrl != "" {
		configuredCredentialManagers = append(configuredCredentialManagers, c.Conjur.Name())
	}

	if c.CredHub.URL != "" || c.CredHub.UAA.ClientId != "" || c.CredHub.UAA.ClientSecret != "" || len(c.CredHub.TLS.CACerts) != 0 || c.CredHub.TLS.ClientCert != "" || c.CredHub.TLS.ClientKey != "" {
		configuredCredentialManagers = append(configuredCredentialManagers, c.CredHub.Name())
	}

	if len(c.Dummy.Vars) > 0 {
		configuredCredentialManagers = append(configuredCredentialManagers, c.Dummy.Name())
	}

	if c.Kubernetes.InClusterConfig || c.Kubernetes.ConfigPath != "" {
		configuredCredentialManagers = append(configuredCredentialManagers, c.Kubernetes.Name())
	}

	if c.SecretsManager.AwsRegion != "" {
		configuredCredentialManagers = append(configuredCredentialManagers, c.SecretsManager.Name())
	}

	if c.SSM.AwsRegion != "" {
		configuredCredentialManagers = append(configuredCredentialManagers, c.SSM.Name())
	}

	if c.Vault.URL != "" {
		configuredCredentialManagers = append(configuredCredentialManagers, c.Vault.Name())
	}

	switch numConfigured := len(configuredCredentialManagers); {
	case numConfigured == 1:
		web.CredentialManager = configuredCredentialManagers[0]
		return nil
	case numConfigured > 1:
		return errors.New(fmt.Sprintf("too many credential managers set: %v", configuredCredentialManagers))
	default:
		return nil
	}
}

func convertMetricEmitterFlags(web *WebConfig) error {
	var configuredMetricEmitters []string

	e := web.Metrics.Emitters
	if e.Datadog.Host != "" && e.Datadog.Port != "" {
		configuredMetricEmitters = append(configuredMetricEmitters, e.Datadog.ID())
	}

	if e.InfluxDB.URL != "" {
		configuredMetricEmitters = append(configuredMetricEmitters, e.InfluxDB.ID())
	}

	if e.Lager.Enabled {
		configuredMetricEmitters = append(configuredMetricEmitters, e.Lager.ID())
	}

	if e.NewRelic.AccountID != "" && e.NewRelic.APIKey != "" {
		configuredMetricEmitters = append(configuredMetricEmitters, e.NewRelic.ID())
	}

	if e.Prometheus.BindPort != "" && e.Prometheus.BindIP != "" {
		configuredMetricEmitters = append(configuredMetricEmitters, e.Prometheus.ID())
	}

	switch numConfigured := len(configuredMetricEmitters); {
	case numConfigured == 1:
		web.Metrics.Emitter = configuredMetricEmitters[0]
		return nil
	case numConfigured > 1:
		return errors.New(fmt.Sprintf("too many metric emitters set: %v", configuredMetricEmitters))
	default:
		return nil
	}
}

func convertTracingProviderFlags(web *WebConfig) error {
	var configuredTracingProviders []string

	t := web.Tracing.Providers
	if t.Honeycomb.APIKey != "" && t.Honeycomb.Dataset != "" {
		configuredTracingProviders = append(configuredTracingProviders, t.Honeycomb.ID())
	}

	if t.Jaeger.Endpoint != "" {
		configuredTracingProviders = append(configuredTracingProviders, t.Jaeger.ID())
	}

	if t.OTLP.Address != "" {
		configuredTracingProviders = append(configuredTracingProviders, t.OTLP.ID())
	}

	if t.Stackdriver.ProjectID != "" {
		configuredTracingProviders = append(configuredTracingProviders, t.Stackdriver.ID())
	}

	switch numConfigured := len(configuredTracingProviders); {
	case numConfigured == 1:
		web.Tracing.Provider = configuredTracingProviders[0]
		return nil
	case numConfigured > 1:
		return errors.New(fmt.Sprintf("too many tracing providers set: %v", configuredTracingProviders))
	default:
		return nil
	}
}

func convertAuthProviderFlags(web *WebConfig) {
	a := web.Auth.AuthFlags.Connectors
	if a.BitbucketCloud.ClientID != "" && a.BitbucketCloud.ClientSecret != "" {
		web.Auth.AuthFlags.Connectors.BitbucketCloud.Enabled = true
	}

	if a.CF.APIURL != "" && a.CF.ClientID != "" && a.CF.ClientSecret != "" {
		web.Auth.AuthFlags.Connectors.CF.Enabled = true
	}

	if a.Github.ClientID != "" && a.Github.ClientSecret != "" {
		web.Auth.AuthFlags.Connectors.Github.Enabled = true
	}

	if a.Gitlab.ClientID != "" && a.Gitlab.ClientSecret != "" {
		web.Auth.AuthFlags.Connectors.Gitlab.Enabled = true
	}

	if a.LDAP.Host != "" && a.LDAP.BindDN != "" && a.LDAP.BindPW != "" {
		web.Auth.AuthFlags.Connectors.LDAP.Enabled = true
	}

	if a.Microsoft.ClientID != "" && a.Microsoft.ClientSecret != "" {
		web.Auth.AuthFlags.Connectors.Microsoft.Enabled = true
	}

	if a.OAuth.AuthURL != "" && a.OAuth.TokenURL != "" && a.OAuth.UserInfoURL != "" && a.OAuth.ClientID != "" && a.OAuth.ClientSecret != "" {
		web.Auth.AuthFlags.Connectors.OAuth.Enabled = true
	}

	if a.SAML.SsoURL != "" && a.SAML.CACert != "" {
		web.Auth.AuthFlags.Connectors.SAML.Enabled = true
	}

	return
}
