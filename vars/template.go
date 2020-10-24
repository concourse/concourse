package vars

import (
	"fmt"
	"sort"

	"sigs.k8s.io/yaml"
)

type Template struct {
	bytes []byte
}

type EvaluateOpts struct {
	ExpectAllKeys bool
}

func NewTemplate(bytes []byte) Template {
	return Template{bytes: bytes}
}

func (t Template) Evaluate(vars Variables, opts EvaluateOpts) ([]byte, error) {
	var document Any
	err := yaml.Unmarshal(t.bytes, &document, UseNumber)
	if err != nil {
		return []byte{}, err
	}

	resolver := newTrackingResolver(vars, opts.ExpectAllKeys)
	obj, err := Interpolate(document, resolver)
	if err != nil {
		return []byte{}, err
	}
	if err := resolver.Error(); err != nil {
		return []byte{}, err
	}

	bytes, err := yaml.Marshal(obj)
	if err != nil {
		return []byte{}, err
	}

	return bytes, nil
}

type trackingResolver struct {
	vars           Variables
	expectAllFound bool
	missing        map[string]struct{}
}

func newTrackingResolver(vars Variables, expectAllFound bool) trackingResolver {
	return trackingResolver{
		vars:           vars,
		expectAllFound: expectAllFound,
		missing:        map[string]struct{}{},
	}
}

func (t trackingResolver) Resolve(ref Reference) (interface{}, error) {
	val, found, err := t.vars.Get(ref)
	if !found || err != nil {
		t.missing[identifier(ref)] = struct{}{}
		return "((" + ref.String() + "))", err
	}

	return val, nil
}

func (t trackingResolver) Error() error {
	if !t.expectAllFound || len(t.missing) == 0 {
		return nil
	}

	return UndefinedVarsError{Vars: names(t.missing)}
}

func names(mapWithNames map[string]struct{}) []string {
	var names []string
	for name := range mapWithNames {
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
