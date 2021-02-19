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
	"github.com/concourse/concourse/flag"
	"github.com/concourse/concourse/tsa/tsacmd"
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/concourse/concourse/atc/atccmd"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
)

var configFile string
var webCmd Web

// type WebCommand struct {
// 	PeerAddress string `long:"peer-address" default:"127.0.0.1" description:"Network address of this web node, reachable by other web nodes. Used for forwarded worker addresses."`

// 	*atccmd.RunConfig
// 	*tsacmd.TSACommand `group:"TSA Configuration" namespace:"tsa"`
// }
var WebCommand = &cobra.Command{
	Use:   "web",
	Short: "TODO",
	Long:  `TODO`,
	RunE:  InitializeWeb,
}

func init() {
	WebCommand.Flags().StringVar(&configFile, "config", "", "config file (default is $HOME/.cobra.yaml)")

	WebCommand.Flags().StringVar(&webCmd.PeerAddress, "peer-address", "127.0.0.1", "Network address of this web node, reachable by other web nodes. Used for forwarded worker addresses.")

	*webCmd.RunConfig = atccmd.CmdDefaults

	// XXX: Can be removed when flags no longer supported
	atccmd.InitializeATCFlagsDEPRECATED(WebCommand, webCmd.RunConfig)
	tsacmd.InitializeFlagsDEPRECATED(WebCommand, webCmd.TSACommand)

	// TODO: Mark all flags as deprecated
}

func InitializeWeb(cmd *cobra.Command, args []string) error {
	// XXX: This can be removed after we completely deprecate flags
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
	if configFile != "" {
		file, err := os.Open(configFile)
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

	webValidator := atccmd.NewValidator(trans)

	err = webValidator.Struct(webCmd)
	if err != nil {
		validationErrors := err.(validator.ValidationErrors)

		// TODO: FIX ERROR HANDLING
		var errOuts []string
		for _, err := range validationErrors {
			errOuts = append(errOuts, err.Translate(trans))
		}

		return fmt.Errorf(`TODO`)
	}

	err = webCmd.Execute(cmd, args)
	if err != nil {
		return fmt.Errorf("failed to execute web: %s", err)
	}

	return nil
}

func fixupFlagDefaults(cmd *cobra.Command, web *Web) {
	// XXX: TEST THIS
	if !cmd.Flags().Changed("default-task-cpu-limit") {
		web.DefaultCpuLimit = nil
	}

	if !cmd.Flags().Changed("default-task-memory-limit") {
		web.DefaultMemoryLimit = nil
	}
}

type Web struct {
	PeerAddress string `yaml:"peer_address"`

	*atccmd.RunConfig
	*tsacmd.TSACommand `yaml:"worker_gateway"`
}

func (w *Web) Execute(cmd *cobra.Command, args []string) error {
	runner, err := w.Runner(args)
	if err != nil {
		return err
	}

	return <-ifrit.Invoke(sigmon.New(runner)).Wait()
}

func (w *Web) Runner(args []string) (ifrit.Runner, error) {
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

func (w *Web) populateSharedFlags() error {
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
		w.TSACommand.ATCURLs = []string{w.RunConfig.DefaultURL().URL.String()}
	}

	if w.TSACommand.TokenURL == "" {
		tokenPath, _ := url.Parse("/sky/issuer/token")
		w.TSACommand.TokenURL = w.RunConfig.DefaultURL().ResolveReference(tokenPath).String()
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

func (w *Web) validateSystemClaimValues() error {
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
