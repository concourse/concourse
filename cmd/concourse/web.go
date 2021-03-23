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
	"github.com/concourse/concourse/flag"
	"github.com/concourse/concourse/tsa/tsacmd"
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
	Short: "Start up web component of Concourse",
	Long: `Concourse relies on the web component to start up the ATC
	and the TSA.`,
	RunE: InitializeWeb,
}

func init() {
	WebCommand.Flags().Var(&webCmd.configFile, "config", "config file (default is $HOME/.cobra.yaml)")

	WebCommand.Flags().StringVar(&webCmd.PeerAddress, "peer-address", "127.0.0.1", "Network address of this web node, reachable by other web nodes. Used for forwarded worker addresses.")

	webCmd.RunConfig = &atccmd.CmdDefaults
	webCmd.TSACommand = &tsacmd.CmdDefaults

	// IMPORTANT!: Can be removed when flags no longer supported
	InitializeATCFlagsDEPRECATED(WebCommand, webCmd.RunConfig)
	InitializeFlagsDEPRECATED(WebCommand, webCmd.TSACommand)

	// TODO: Mark all flags as deprecated
}

type WebConfig struct {
	configFile flag.File

	PeerAddress string `yaml:"peer_address"`

	*atccmd.RunConfig  `yaml:"web" ignore_env:"true"`
	*tsacmd.TSACommand `yaml:"worker_gateway"`
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

		Parser: envstruct.Parser{
			Delimiter:   ",",
			Unmarshaler: yaml.Unmarshal,
		},
	}

	err := env.FetchEnv(webCmd)
	if err != nil {
		return fmt.Errorf("fetch env: %s", err)
	}

	// Fetch out the values set from the config file and overwrite the flag
	// values
	if webCmd.configFile != "" {
		file, err := os.Open(string(webCmd.configFile))
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

func fixupFlagDefaults(cmd *cobra.Command, web *WebConfig) {
	// XXX: TEST THIS
	if !cmd.Flags().Changed("default-task-cpu-limit") {
		web.DefaultCpuLimit = nil
	}

	if !cmd.Flags().Changed("default-task-memory-limit") {
		web.DefaultMemoryLimit = nil
	}
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

	tsaRunner, err := w.TSACommand.Runner(args)
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

	w.TSACommand.PeerAddress = w.PeerAddress

	if len(w.TSACommand.ATCURLs) == 0 {
		w.TSACommand.ATCURLs = flag.URLs{w.RunConfig.DefaultURL()}
	}

	if w.TSACommand.TokenURL.URL == nil {
		tokenPath, _ := url.Parse("/sky/issuer/token")
		w.TSACommand.TokenURL.URL = w.RunConfig.DefaultURL().URL.ResolveReference(tokenPath)
	}

	if w.TSACommand.ClientSecret == "" {
		w.TSACommand.ClientSecret = derivedCredential(signingKey, w.TSACommand.ClientID)
	}

	if w.RunConfig.Server.ClientSecret == "" {
		w.RunConfig.Server.ClientSecret = derivedCredential(signingKey, w.RunConfig.Server.ClientID)
	}

	w.RunConfig.Auth.AuthFlags.Clients[w.TSACommand.ClientID] = w.TSACommand.ClientSecret
	w.RunConfig.Auth.AuthFlags.Clients[w.RunConfig.Server.ClientID] = w.RunConfig.Server.ClientSecret

	// if we're using the 'aud' as the SystemClaimKey then we want to validate
	// that the SystemClaimValues contains our TSA Client. If it's not 'aud' then
	// we can't validate anything
	if w.RunConfig.SystemClaim.Key == "aud" {

		// if we're using the default SystemClaimValues then override these values
		// to make sure they include the TSA ClientID
		if len(w.RunConfig.SystemClaim.Values) == 1 {
			if w.RunConfig.SystemClaim.Values[0] == "concourse-worker" {
				w.RunConfig.SystemClaim.Values = []string{w.TSACommand.ClientID}
			}
		}

		if err := w.validateSystemClaimValues(); err != nil {
			return err
		}
	}

	w.TSACommand.ClusterName = w.RunConfig.Server.ClusterName
	w.TSACommand.LogClusterName = w.RunConfig.Log.ClusterName

	return nil
}

func (w *WebConfig) validateSystemClaimValues() error {
	found := false
	for _, val := range w.RunConfig.SystemClaim.Values {
		if val == w.TSACommand.ClientID {
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
