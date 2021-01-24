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
	"github.com/concourse/concourse/tsa/tsacmd"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/concourse/concourse/atc/atccmd"
	"github.com/concourse/flag"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
)

var configFile string
var webFlagsDEPRECATED Web

type fieldError struct {
	err validator.FieldError
}

func (q fieldError) String() string {
	errorString := fmt.Sprintf("validation failed on field '%s', condition: %s", q.err.Field(), q.err.ActualTag())

	// Print condition parameters, e.g. oneof=red blue -> { red blue }
	if q.err.Param() != "" {
		sb.WriteString(" { " + q.err.Param() + " }")
	}

	if q.err.Value() != nil && q.err.Value() != "" {
		sb.WriteString(fmt.Sprintf(", actual: %v", q.err.Value()))
	}

	return sb.String()
}

// type WebCommand struct {
// 	PeerAddress string `long:"peer-address" default:"127.0.0.1" description:"Network address of this web node, reachable by other web nodes. Used for forwarded worker addresses."`

// 	*atccmd.RunCommand
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

	WebCommand.Flags().StringVar(&webFlagsDEPRECATED.PeerAddress, "peer-address", "127.0.0.1", "Network address of this web node, reachable by other web nodes. Used for forwarded worker addresses.")

	atccmd.InitializeFlagsDEPRECATED(WebCommand, webFlagsDEPRECATED.RunCommand)

	tsacmd.InitializeFlagsDEPRECATED(WebCommand, webFlagsDEPRECATED.TSACommand)

	// Mark all flags as deprecated
}

// XXX: Double check with alex that we no longer need these
// func (WebCommand) LessenRequirements(command *flags.Command) {
// }

func InitializeWeb(cmd *cobra.Command, args []string) error {
	// Fetch all the flag values set
	//
	// XXX: When we stop supporting flags, we will need to replace this with a
	// new web object and fill in defaults manually with:
	// atccmd.SetDefaults(web.RunCommand)
	// tsacmd.SetDefaults(web.TSACommand)
	web := webFlagsDEPRECATED

	// IMPORTANT!! This can be removed after we completely deprecate flags
	fixupFlagDefaults(cmd, &web)

	// Fetch out env values
	env := envstruct.New("CONCOURSE", "yaml", envstruct.Parser{
		Delimiter:   ",",
		Unmarshaler: yaml.Unmarshal,
	})

	err := env.FetchEnv(web)
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
		err = decoder.Decode(&web)
		if err != nil {
			return fmt.Errorf("decode config: %s", err)
		}
	}

	// Validate the values passed in by the user
	webValidator := atccmd.NewValidator()
	err = webValidator.Struct(web)
	if err != nil {
		validationErrors := err.(validator.ValidationErrors)

		var errOuts []string
		for _, err := range validationErrors {
			errOuts = append(errOuts, err)
		}
		// XXX: TODO ERROR

		return fmt.Errorf(`TODO`)
	}

	err = web.Execute(cmd, args)
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

	*atccmd.RunCommand
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
	if w.RunCommand.CLIArtifactsDir == "" {
		w.RunCommand.CLIArtifactsDir = concourseCmd.DiscoverAsset("fly-assets")
	}

	err := w.populateSharedFlags()
	if err != nil {
		return nil, err
	}

	atcRunner, err := w.RunCommand.Runner(args)
	if err != nil {
		return nil, err
	}

	tsaRunner, err := w.TSACommand.Runner(args)
	if err != nil {
		return nil, err
	}

	logger, _ := w.RunCommand.Logger.Logger("web")
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
	if w.RunCommand.Auth.AuthFlags.SigningKey == nil || w.RunCommand.Auth.AuthFlags.SigningKey.PrivateKey == nil {
		var err error
		signingKey, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return fmt.Errorf("failed to generate session signing key: %s", err)
		}

		w.RunCommand.Auth.AuthFlags.SigningKey = &flag.PrivateKey{PrivateKey: signingKey}
	} else {
		signingKey = w.RunCommand.Auth.AuthFlags.SigningKey.PrivateKey
	}

	w.TSACommand.PeerAddress = w.PeerAddress

	if len(w.TSACommand.ATCURLs) == 0 {
		w.TSACommand.ATCURLs = []string{w.RunCommand.DefaultURL().String()}
	}

	if w.TSACommand.TokenURL == "" {
		tokenPath, _ := url.Parse("/sky/issuer/token")
		w.TSACommand.TokenURL = w.RunCommand.DefaultURL().ResolveReference(tokenPath).String()
	}

	if w.TSACommand.ClientSecret == "" {
		w.TSACommand.ClientSecret = derivedCredential(signingKey, w.TSACommand.ClientID)
	}

	if w.RunCommand.Server.ClientSecret == "" {
		w.RunCommand.Server.ClientSecret = derivedCredential(signingKey, w.RunCommand.Server.ClientID)
	}

	w.RunCommand.Auth.AuthFlags.Clients[w.TSACommand.ClientID] = w.TSACommand.ClientSecret
	w.RunCommand.Auth.AuthFlags.Clients[w.RunCommand.Server.ClientID] = w.RunCommand.Server.ClientSecret

	// if we're using the 'aud' as the SystemClaimKey then we want to validate
	// that the SystemClaimValues contains our TSA Client. If it's not 'aud' then
	// we can't validate anything
	if w.RunCommand.SystemClaim.Key == "aud" {

		// if we're using the default SystemClaimValues then override these values
		// to make sure they include the TSA ClientID
		if len(w.RunCommand.SystemClaim.Values) == 1 {
			if w.RunCommand.SystemClaim.Values[0] == "concourse-worker" {
				w.RunCommand.SystemClaim.Values = []string{w.TSACommand.ClientID}
			}
		}

		if err := w.validateSystemClaimValues(); err != nil {
			return err
		}
	}

	w.TSACommand.ClusterName = w.RunCommand.Server.ClusterName
	w.TSACommand.LogClusterName = w.RunCommand.Log.ClusterName

	return nil
}

func (w *Web) validateSystemClaimValues() error {
	found := false
	for _, val := range w.RunCommand.SystemClaim.Values {
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
