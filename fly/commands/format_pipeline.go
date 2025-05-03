package commands

import (
	"fmt"
	"os"
	"regexp"

	"sigs.k8s.io/yaml"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
)

// placeholderWrapper handles YAML placeholder wrapping and unwrapping
type placeholderWrapper struct {
	left  string
	right string
}

// newPlaceholderWrapper creates a wrapper for the given placeholder delimiters
func newPlaceholderWrapper(left, right string) *placeholderWrapper {
	return &placeholderWrapper{
		left:  left,
		right: right,
	}
}

// wrap placeholders in single quotes to make YAML valid
func (w *placeholderWrapper) wrap(input []byte) []byte {
	pattern := `\s` + regexp.QuoteMeta(w.left) + `([^` + regexp.QuoteMeta(w.right) + `]+)` + regexp.QuoteMeta(w.right)
	re := regexp.MustCompile(pattern)

	if !re.Match(input) {
		return input
	}

	return re.ReplaceAll(input, []byte(fmt.Sprintf(` '%s$1%s'`, w.left, w.right)))
}

// unwrap removes quotes from placeholders to restore original format
func (w *placeholderWrapper) unwrap(input []byte) []byte {
	pattern := `\s'` + regexp.QuoteMeta(w.left) + `([^` + regexp.QuoteMeta(w.right) + `]+)` + regexp.QuoteMeta(w.right) + `'`
	re := regexp.MustCompile(pattern)

	if !re.Match(input) {
		return input
	}

	return re.ReplaceAll(input, []byte(fmt.Sprintf(` %s$1%s`, w.left, w.right)))
}

type FormatPipelineCommand struct {
	Config atc.PathFlag `short:"c" long:"config" required:"true" description:"Pipeline configuration file"`
	Write  bool         `short:"w" long:"write" description:"Do not print to stdout; overwrite the file in place"`
}

func (command *FormatPipelineCommand) Execute(args []string) error {
	configPath := string(command.Config)
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		displayhelpers.FailWithErrorf("could not read config file", err)
	}

	wrapper := newPlaceholderWrapper("{{", "}}")

	// Format the YAML while preserving placeholders
	wrappedConfigBytes := wrapper.wrap(configBytes)

	var config atc.Config
	err = yaml.Unmarshal(wrappedConfigBytes, &config)
	if err != nil {
		displayhelpers.FailWithErrorf("could not unmarshal config", err)
	}

	formattedBytes, err := yaml.Marshal(config)
	if err != nil {
		displayhelpers.FailWithErrorf("could not marshal config", err)
	}

	unwrappedConfigBytes := wrapper.unwrap(formattedBytes)

	if command.Write {
		fi, err := os.Stat(configPath)
		if err != nil {
			displayhelpers.FailWithErrorf("could not stat config file", err)
		}

		err = os.WriteFile(configPath, unwrappedConfigBytes, fi.Mode())
		if err != nil {
			displayhelpers.FailWithErrorf("could not write formatted config to %s", err, command.Config)
		}
	} else {
		_, err = fmt.Fprint(os.Stdout, string(unwrappedConfigBytes))
		if err != nil {
			displayhelpers.FailWithErrorf("could not write formatted config to stdout", err)
		}
	}

	return nil
}
