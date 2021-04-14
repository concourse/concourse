package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/clarafu/envstruct"
	v "github.com/concourse/concourse/cmd/concourse/validator"
	"github.com/concourse/concourse/worker/workercmd"
	"github.com/concourse/flag"
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var workerCmd WorkerConfig

var WorkerCommand = &cobra.Command{
	Use:   "worker",
	Short: "Run and register a worker",
	Long:  `TODO`,
	RunE:  InitializeWorker,
}

type WorkerConfig struct {
	ConfigFile flag.File `env:"CONCOURSE_WORKER_CONFIG_FILE"`

	workercmd.WorkerCommand `yaml:"worker" ignore_env:"true"`
}

func init() {
	WorkerCommand.Flags().Var(&workerCmd.ConfigFile, "config", "config file (default is $HOME/.cobra.yaml)")

	workerCmd.WorkerCommand = workercmd.CmdDefaults
	workercmd.InitializeWorkerFlagsDEPRECATED(WorkerCommand, &workerCmd.WorkerCommand, "")
}

func InitializeWorker(cmd *cobra.Command, args []string) error {
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

	err := env.FetchEnv(&workerCmd)
	if err != nil {
		return fmt.Errorf("fetch env: %s", err)
	}

	// Fetch out the values set from the config file and overwrite the flag
	// values
	if workerCmd.ConfigFile != "" {
		file, err := os.Open(string(workerCmd.ConfigFile))
		if err != nil {
			return fmt.Errorf("open file: %s", err)
		}

		decoder := yaml.NewDecoder(file)
		err = decoder.Decode(&workerCmd)
		if err != nil {
			return fmt.Errorf("decode config: %s", err)
		}
	}

	// Validate the values passed in by the user
	en := en.New()
	uni := ut.New(en, en)
	trans, _ := uni.GetTranslator("en")

	validate := v.NewValidator(trans)

	err = validate.Struct(workerCmd)
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

	err = workerCmd.Execute(args)
	if err != nil {
		return fmt.Errorf("failed to execute web: %s", err)
	}

	return nil
}
