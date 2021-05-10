package main

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"os"

	"github.com/clarafu/envstruct"
	"github.com/concourse/concourse/atc/atccmd"
	concourseCmd "github.com/concourse/concourse/cmd"
	v "github.com/concourse/concourse/cmd/concourse/validator"
	"github.com/concourse/concourse/tsa/tsacmd"
	"github.com/concourse/concourse/worker/workercmd"
	"github.com/concourse/flag"
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
)

var quickStart QuickstartConfig

var QuickstartCommand = &cobra.Command{
	Use:   "quickstart",
	Short: "TODO",
	Long: `Run both 'web' and 'worker' together, auto-wired.
	Not recommended for productionTBD`,
	RunE: InitializeQuickstart,
}

func init() {
	QuickstartCommand.Flags().StringVar(&quickStart.ConfigFile, "config", "", "path to the config file that will be used to quickstart the concourse cluster")

	quickStart.WebConfig.RunConfig = atccmd.CmdDefaults
	quickStart.WebConfig.TSAConfig = tsacmd.CmdDefaults
	quickStart.WorkerCommand = workercmd.CmdDefaults

	// IMPORTANT!: Can be removed when flags no longer supported
	atccmd.InitializeATCFlagsDEPRECATED(QuickstartCommand, &quickStart.WebConfig.RunConfig)
	tsacmd.InitializeTSAFlagsDEPRECATED(QuickstartCommand, &quickStart.WebConfig.TSAConfig)
	workercmd.InitializeWorkerFlagsDEPRECATED(QuickstartCommand, &quickStart.WorkerCommand, "worker-")
}

type QuickstartConfig struct {
	ConfigFile string `env:"QUICKSTART_CONFIG_FILE"`

	WebConfig               `yaml:"web" ignore_env:"true"`
	workercmd.WorkerCommand `yaml:"worker"`
}

func InitializeQuickstart(cmd *cobra.Command, args []string) error {
	// IMPORTANT! This can be removed after we completely deprecate flags
	fixupFlagDefaults(cmd, &quickStart.WebConfig)

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

	err := env.FetchEnv(&quickStart)
	if err != nil {
		return fmt.Errorf("fetch env: %s", err)
	}

	// Fetch out the values set from the config file and overwrite the flag
	// values
	if quickStart.ConfigFile != "" {
		file, err := os.Open(quickStart.ConfigFile)
		if err != nil {
			return fmt.Errorf("open file: %s", err)
		}

		decoder := yaml.NewDecoder(file)
		err = decoder.Decode(&quickStart)
		if err != nil {
			return fmt.Errorf("decode config: %s", err)
		}
	}

	err = quickStart.PopulateFields()
	if err != nil {
		return fmt.Errorf("populate fields: %s", err)
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

	err = quickStart.Execute(args)
	if err != nil {
		return fmt.Errorf("failed to execute web: %s", err)
	}

	return nil
}

func (cmd *QuickstartConfig) Execute(args []string) error {
	runner, err := cmd.Runner(args)
	if err != nil {
		return err
	}

	return <-ifrit.Invoke(sigmon.New(runner)).Wait()
}

func checkNilKeys(key flag.PrivateKey) bool {
	if key.PrivateKey == nil {
		return true
	}
	return false
}

func (cmd *QuickstartConfig) Runner(args []string) (ifrit.Runner, error) {
	webRunner, err := cmd.WebConfig.Runner(args)
	if err != nil {
		return nil, err
	}

	workerRunner, err := cmd.WorkerCommand.Runner(args)
	if err != nil {
		return nil, err
	}

	logger, _ := cmd.WebConfig.RunConfig.Logger.Logger("quickstart")
	return grouper.NewParallel(os.Interrupt, grouper.Members{
		{
			Name:   "web",
			Runner: concourseCmd.NewLoggingRunner(logger.Session("web-runner"), webRunner)},
		{
			Name:   "worker",
			Runner: concourseCmd.NewLoggingRunner(logger.Session("worker-runner"), workerRunner)},
	}), nil
}

func (cmd *QuickstartConfig) PopulateFields() error {
	if checkNilKeys(cmd.WebConfig.TSAConfig.HostKey) {
		tsaHostKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return fmt.Errorf("failed to generate tsa host key: %s", err)
		}

		tsaHostPublicKey, err := ssh.NewPublicKey(tsaHostKey.Public())
		if err != nil {
			return fmt.Errorf("failed to create worker authorized key: %s", err)
		}

		cmd.WebConfig.TSAConfig.HostKey = flag.PrivateKey{PrivateKey: tsaHostKey}
		cmd.WorkerCommand.TSA.PublicKey.Keys =
			append(cmd.WorkerCommand.TSA.PublicKey.Keys, tsaHostPublicKey)
	}

	if checkNilKeys(cmd.WorkerCommand.TSA.WorkerPrivateKey) {
		workerKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return fmt.Errorf("failed to generate worker key: %s", err)
		}

		workerPublicKey, err := ssh.NewPublicKey(workerKey.Public())
		if err != nil {
			return fmt.Errorf("failed to create worker authorized key: %s", err)
		}

		cmd.WorkerCommand.TSA.WorkerPrivateKey = flag.PrivateKey{PrivateKey: workerKey}
		cmd.WebConfig.TSAConfig.AuthorizedKeys.Keys =
			append(cmd.WebConfig.TSAConfig.AuthorizedKeys.Keys, workerPublicKey)
	}

	return nil
}
