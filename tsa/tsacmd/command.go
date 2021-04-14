package tsacmd

import (
	"errors"
	"fmt"
	"os"

	v "github.com/concourse/concourse/cmd/concourse/validator"
	"github.com/concourse/flag"
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	val "github.com/go-playground/validator/v10"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"
)

var tsaCmd TSACommandFlags

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
	TSACommand.Flags().Var(&tsaCmd.ConfigFile, "config", "config file (default is $HOME/.cobra.yaml)")

	tsaCmd.TSAConfig = CmdDefaults

	InitializeTSAFlagsDEPRECATED(TSACommand, &tsaCmd.TSAConfig)
}

type TSACommandFlags struct {
	ConfigFile flag.File `env:"TSA_CONFIG_FILE"`

	TSAConfig
}

func InitializeTSA(cmd *cobra.Command, args []string) error {
	// Fetch out the values set from the config file and overwrite the flag
	// values
	if tsaCmd.ConfigFile != "" {
		file, err := os.Open(string(tsaCmd.ConfigFile))
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

	err := validator.Struct(tsaCmd)
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

