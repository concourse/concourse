package vars

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type Template struct {
	bytes []byte
}

type EvaluateOpts struct {
	ExpectAllKeys     bool
	ExpectAllVarsUsed bool
}

func NewTemplate(bytes []byte) Template {
	return Template{bytes: bytes}
}

func (t Template) Evaluate(vars Variables, opts EvaluateOpts) ([]byte, error) {
	var obj interface{}

	err := yaml.Unmarshal(t.bytes, &obj)
	if err != nil {
		return []byte{}, err
	}

	obj, err = t.interpolateRoot(obj, newVarsTracker(vars, opts.ExpectAllKeys, opts.ExpectAllVarsUsed))
	if err != nil {
		return []byte{}, err
	}

	bytes, err := yaml.Marshal(obj)
	if err != nil {
		return []byte{}, err
	}

	return bytes, nil
}

func (t Template) interpolateRoot(obj interface{}, tracker varsTracker) (interface{}, error) {
	var err error
	obj, err = interpolator{}.Interpolate(obj, varsLookup{tracker})
	if err != nil {
		return nil, err
	}

	return obj, tracker.Error()
}

type interpolator struct{}

var (
	interpolationRegex         = regexp.MustCompile(`\(\((!?[-/\.\w\pL]+)\)\)`)
	interpolationAnchoredRegex = regexp.MustCompile("\\A" + interpolationRegex.String() + "\\z")
)

func (i interpolator) Interpolate(node interface{}, varsLookup varsLookup) (interface{}, error) {
	switch typedNode := node.(type) {
	case map[interface{}]interface{}:
		for k, v := range typedNode {
			evaluatedValue, err := i.Interpolate(v, varsLookup)
			if err != nil {
				return nil, err
			}

			evaluatedKey, err := i.Interpolate(k, varsLookup)
			if err != nil {
				return nil, err
			}

			delete(typedNode, k) // delete in case key has changed
			typedNode[evaluatedKey] = evaluatedValue
		}

	case []interface{}:
		for idx, x := range typedNode {
			var err error
			typedNode[idx], err = i.Interpolate(x, varsLookup)
			if err != nil {
				return nil, err
			}
		}

	case string:
		for _, name := range i.extractVarNames(typedNode) {
			foundVal, found, err := varsLookup.Get(name)
			if err != nil {
				return nil, errors.WithMessagef(err, "Finding variable '%s'", name)
			}

			if found {
				// ensure that value type is preserved when replacing the entire field
				if interpolationAnchoredRegex.MatchString(typedNode) {
					return foundVal, nil
				}

				switch foundVal.(type) {
				case string, int, int16, int32, int64, uint, uint16, uint32, uint64:
					foundValStr := fmt.Sprintf("%v", foundVal)
					typedNode = strings.Replace(typedNode, fmt.Sprintf("((%s))", name), foundValStr, -1)
					typedNode = strings.Replace(typedNode, fmt.Sprintf("((!%s))", name), foundValStr, -1)
				default:
					return nil, InvalidInterpolationError{
						Path:  name,
						Value: foundVal,
					}
				}
			}
		}

		return typedNode, nil
	}

	return node, nil
}

func (i interpolator) extractVarNames(value string) []string {
	var names []string

	for _, match := range interpolationRegex.FindAllSubmatch([]byte(value), -1) {
		names = append(names, strings.TrimPrefix(string(match[1]), "!"))
	}

	return names
}

type varsLookup struct {
	varsTracker
}

var ErrEmptyVar = errors.New("empty var")

func (l varsLookup) Get(name string) (interface{}, bool, error) {
	splitName := strings.Split(name, ".")

	// this should be impossible since interpolationRegex only matches non-empty
	// vars, but better to error than to panic
	if len(splitName) == 0 {
		return nil, false, ErrEmptyVar
	}

	val, found, err := l.varsTracker.Get(splitName[0])
	if !found || err != nil {
		return val, found, err
	}

	for _, seg := range splitName[1:] {
		switch v := val.(type) {
		case map[interface{}]interface{}:
			var found bool
			val, found = v[seg]
			if !found {
				return nil, false, MissingFieldError{
					Path:  name,
					Field: seg,
				}
			}
		case map[string]interface{}:
			var found bool
			val, found = v[seg]
			if !found {
				return nil, false, MissingFieldError{
					Path:  name,
					Field: seg,
				}
			}
		default:
			return nil, false, InvalidFieldError{
				Path:  name,
				Field: seg,
				Value: val,
			}
		}
	}

	return val, true, err
}

type varsTracker struct {
	vars Variables

	expectAllFound bool
	expectAllUsed  bool

	missing    map[string]struct{} // track missing var names
	visited    map[string]struct{}
	visitedAll map[string]struct{} // track all var names that were accessed
}

func newVarsTracker(vars Variables, expectAllFound, expectAllUsed bool) varsTracker {
	return varsTracker{
		vars:           vars,
		expectAllFound: expectAllFound,
		expectAllUsed:  expectAllUsed,
		missing:        map[string]struct{}{},
		visited:        map[string]struct{}{},
		visitedAll:     map[string]struct{}{},
	}
}

func (t varsTracker) Get(name string) (interface{}, bool, error) {
	t.visitedAll[name] = struct{}{}

	val, found, err := t.vars.Get(VariableDefinition{Name: name})
	if !found {
		t.missing[name] = struct{}{}
	}

	return val, found, err
}

func (t varsTracker) Error() error {
	missingErr := t.MissingError()
	extraErr := t.ExtraError()
	if missingErr != nil && extraErr != nil {
		return multierror.Append(missingErr, extraErr)
	} else if missingErr != nil {
		return missingErr
	} else if extraErr != nil {
		return extraErr
	}

	return nil
}

func (t varsTracker) MissingError() error {
	if !t.expectAllFound || len(t.missing) == 0 {
		return nil
	}

	return UndefinedVarsError{Vars: names(t.missing)}
}

func (t varsTracker) ExtraError() error {
	if !t.expectAllUsed {
		return nil
	}

	allDefs, err := t.vars.List()
	if err != nil {
		return err
	}

	unusedNames := map[string]struct{}{}

	for _, def := range allDefs {
		if _, found := t.visitedAll[def.Name]; !found {
			unusedNames[def.Name] = struct{}{}
		}
	}

	if len(unusedNames) == 0 {
		return nil
	}

	return UnusedVarsError{Vars: names(unusedNames)}
}

func names(mapWithNames map[string]struct{}) []string {
	var names []string
	for name, _ := range mapWithNames {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}
