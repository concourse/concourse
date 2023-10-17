package vars

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/go-multierror"
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

func (t Template) ExtraVarNames() []string {
	return interpolator{}.extractVarNames(string(t.bytes))
}

func (t Template) Evaluate(vars Variables, opts EvaluateOpts) ([]byte, error) {
	var obj interface{}

	// Note: if we do end up changing from "gopkg.in/yaml.v2" to
	// "sigs.k8s.io/yaml" here, we'll want to ensure we call
	// `json.Decoder.UseNumber()` so that we don't lose precision unmarshaling
	// numbers to float64.
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
	obj, err = interpolator{}.Interpolate(obj, tracker)
	if err != nil {
		return nil, err
	}

	return obj, tracker.Error()
}

type interpolator struct{}

var (
	interpolationRegex         = regexp.MustCompile(`\(\((([-/\.\w\pL]+\:)?[-/\.:@"\w\pL]+)\)\)`)
	interpolationAnchoredRegex = regexp.MustCompile("\\A" + interpolationRegex.String() + "\\z")
)

func (i interpolator) Interpolate(node interface{}, tracker varsTracker) (interface{}, error) {
	switch typedNode := node.(type) {
	case map[interface{}]interface{}:
		for k, v := range typedNode {
			evaluatedValue, err := i.Interpolate(v, tracker)
			if err != nil {
				return nil, err
			}

			evaluatedKey, err := i.Interpolate(k, tracker)
			if err != nil {
				return nil, err
			}

			delete(typedNode, k) // delete in case key has changed
			typedNode[evaluatedKey] = evaluatedValue
		}

	case []interface{}:
		for idx, x := range typedNode {
			var err error
			typedNode[idx], err = i.Interpolate(x, tracker)
			if err != nil {
				return nil, err
			}
		}

	case string:
		for _, name := range i.extractVarNames(typedNode) {
			foundVal, found, err := tracker.Get(name)
			if err != nil {
				return nil, err
			}

			if found {
				// ensure that value type is preserved when replacing the entire field
				if interpolationAnchoredRegex.MatchString(typedNode) {
					return foundVal, nil
				}

				switch foundVal.(type) {
				case string, int, int16, int32, int64, uint, uint16, uint32, uint64, json.Number:
					foundValStr := fmt.Sprintf("%v", foundVal)
					typedNode = strings.Replace(typedNode, fmt.Sprintf("((%s))", name), foundValStr, -1)
				default:
					return nil, InvalidInterpolationError{
						Name:  name,
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
		names = append(names, string(match[1]))
	}

	return names
}

type varsTracker struct {
	vars Variables

	expectAllFound bool
	expectAllUsed  bool

	missing    map[string]struct{}
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

// Get value of a var. Name can be the following formats: 1) 'foo', where foo
// is var name; 2) 'foo:bar', where foo is var source name, and bar is var name;
// 3) '.:foo', where . means a local var, foo is var name.
func (t varsTracker) Get(varName string) (interface{}, bool, error) {
	varRef, err := ParseReference(varName)
	if err != nil {
		return nil, false, err
	}

	t.visitedAll[identifier(varRef)] = struct{}{}

	val, found, err := t.vars.Get(varRef)
	if !found || err != nil {
		t.missing[varRef.String()] = struct{}{}
		return val, found, err
	}

	return val, true, err
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

	allRefs, err := t.vars.List()
	if err != nil {
		return err
	}

	unusedNames := map[string]struct{}{}

	for _, ref := range allRefs {
		id := identifier(ref)
		if _, found := t.visitedAll[id]; !found {
			unusedNames[id] = struct{}{}
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

func identifier(varRef Reference) string {
	id := varRef.Path

	if varRef.Source != "" {
		id = fmt.Sprintf("%s:%s", varRef.Source, id)
	}

	return id
}
