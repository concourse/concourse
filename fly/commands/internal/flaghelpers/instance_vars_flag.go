package flaghelpers

import (
	"errors"
	"fmt"
	"strings"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/vars"
	"sigs.k8s.io/yaml"
)

type InstanceVarsFlag struct {
	InstanceVars atc.InstanceVars
}

func (flag *InstanceVarsFlag) UnmarshalFlag(value string) error {
	var err error
	flag.InstanceVars, err = unmarshalInstanceVars(value)
	if err != nil {
		return err
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
		return nil, errors.New("instance vars should be formatted as <key1:value1>(,<key2:value2>)")
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
