package flaghelpers

import (
	"errors"
	"fmt"
	"strings"

	"github.com/concourse/concourse/vars"
	"github.com/jessevdk/go-flags"
	"sigs.k8s.io/yaml"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/go-concourse/concourse"
)

type PipelineFlag struct {
	Name         string
	InstanceVars atc.InstanceVars
}

func (flag *PipelineFlag) Validate() ([]concourse.ConfigWarning, error) {
	var warnings []concourse.ConfigWarning
	if flag != nil {
		if strings.Contains(flag.Name, "/") {
			return nil, errors.New("pipeline name cannot contain '/'")
		}

		warning, err := atc.ValidateIdentifier(flag.Name, "pipeline")
		if err != nil {
			return nil, errors.New("pipeline name cannot contain '/'")
		}
		if warning != nil {
			warnings = append(warnings, concourse.ConfigWarning{
				Type:    warning.Type,
				Message: warning.Message,
			})
		}
	}
	return warnings, nil
}

func (flag *PipelineFlag) Ref() atc.PipelineRef {
	return atc.PipelineRef{Name: flag.Name, InstanceVars: flag.InstanceVars}
}

func (flag *PipelineFlag) UnmarshalFlag(value string) error {
	if !strings.Contains(value, "/") {
		flag.Name = value
		return nil
	}

	vs := strings.SplitN(value, "/", 2)
	if len(vs) == 2 {
		flag.Name = vs[0]
		var err error
		flag.InstanceVars, err = unmarshalInstanceVars(vs[1])
		if err != nil {
			return err
		}
	}
	return nil
}

func unmarshalInstanceVars(s string) (atc.InstanceVars, error) {
	var kvPairs vars.KVPairs
	for {
		colonIndex, ok := findUnquoted(s, `"`, nextOccurrenceOf(':'))
		if !ok {
			break
		}
		rawKey := s[:colonIndex]
		var kvPair vars.KVPair
		var err error
		kvPair.Ref, err = vars.ParseReference(rawKey)
		if err != nil {
			return nil, err
		}

		s = s[colonIndex+1:]
		rawValue := []byte(s)
		commaIndex, hasComma := findUnquoted(s, `"'`, nextOccurrenceOfOutsideOfYAML(','))
		if hasComma {
			rawValue = rawValue[:commaIndex]
			s = s[commaIndex+1:]
		}

		if err := yaml.Unmarshal(rawValue, &kvPair.Value, useNumber); err != nil {
			return nil, fmt.Errorf("invalid value for key '%s': %w", rawKey, err)
		}
		kvPairs = append(kvPairs, kvPair)

		if !hasComma {
			break
		}
	}
	if len(kvPairs) == 0 {
		return nil, errors.New("argument format should be <pipeline>/<key:value>")
	}

	return atc.InstanceVars(kvPairs.Expand()), nil
}

func findUnquoted(s string, quoteset string, stop func(c rune) bool) (int, bool) {
	var quoteChar rune
	for i, c := range s {
		if quoteChar == 0 {
			if stop(c) {
				return i, true
			}
			if strings.ContainsRune(quoteset, c) {
				quoteChar = c
			}
		} else if c == quoteChar {
			quoteChar = 0
		}
	}
	return 0, false
}

func nextOccurrenceOf(r rune) func(rune) bool {
	return func(c rune) bool {
		return c == r
	}
}

func nextOccurrenceOfOutsideOfYAML(r rune) func(rune) bool {
	braceCount := 0
	bracketCount := 0
	return func(c rune) bool {
		switch c {
		case r:
			if braceCount == 0 && bracketCount == 0 {
				return true
			}
		case '{':
			braceCount++
		case '}':
			braceCount--
		case '[':
			bracketCount++
		case ']':
			bracketCount--
		}
		return false
	}
}

func (flag *PipelineFlag) Complete(match string) []flags.Completion {
	fly := parseFlags()

	target, err := rc.LoadTarget(fly.Target, false)
	if err != nil {
		return []flags.Completion{}
	}

	err = target.Validate()
	if err != nil {
		return []flags.Completion{}
	}

	pipelines, err := target.Team().ListPipelines()
	if err != nil {
		return []flags.Completion{}
	}

	comps := []flags.Completion{}
	for _, pipeline := range pipelines {
		if strings.HasPrefix(pipeline.Ref().String(), match) {
			comps = append(comps, flags.Completion{Item: pipeline.Ref().String()})
		}
	}

	return comps
}
