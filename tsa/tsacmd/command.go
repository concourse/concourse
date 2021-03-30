package tsacmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/clarafu/envstruct"
	v "github.com/concourse/concourse/cmd/concourse/validator"
	"github.com/concourse/flag"
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	val "github.com/go-playground/validator/v10"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"
)

var tsaCmd TSAConfig
var configFile flag.File

// TSA command is only used for when the user wants to run the tsa
// independently from the web command. This is not included in the concourse
// commands
var TSACommand = &cobra.Command{
	Use:   "tsa",
	Short: "TODO",
	Long:  `TODO`,
	RunE:  InitializeTSA,
}

func init() {
	TSACommand.Flags().Var(&configFile, "config", "config file (default is $HOME/.cobra.yaml)")

	InitializeTSAFlagsDEPRECATED(TSACommand, &tsaCmd)
}

func InitializeTSA(cmd *cobra.Command, args []string) error {
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

	err := env.FetchEnv(&tsaCmd)
	if err != nil {
		return fmt.Errorf("fetch env: %s", err)
	}

	// Fetch out the values set from the config file and overwrite the flag
	// values
	if configFile != "" {
		file, err := os.Open(string(configFile))
		if err != nil {
			return fmt.Errorf("open file: %s", err)
		}

		decoder := yaml.NewDecoder(file)
		err = decoder.Decode(&tsaCmd)
		if err != nil {
			return fmt.Errorf("decode config: %s", err)
		}
	}

	// Validate the values passed in by the user
	en := en.New()
	uni := ut.New(en, en)
	trans, _ := uni.GetTranslator("en")

	validator := v.NewValidator(trans)

	err = validator.Struct(tsaCmd)
	if err != nil {
		validationErrors := err.(val.ValidationErrors)

		var errs *multierror.Error
		for _, validationErr := range validationErrors {
			errs = multierror.Append(
				errs,
				errors.New(validationErr.Translate(trans)),
			)
		}

		return errs.ErrorOrNil()
	}

	err = tsaCmd.Execute(args)
	if err != nil {
		return fmt.Errorf("failed to execute web: %s", err)
	}

	return nil
}

