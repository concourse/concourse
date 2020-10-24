package vars

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/go-multierror"
)

var (
	interpolationRegex         = regexp.MustCompile(`\(\((([-/.\w\pL]+:)?[-/.:@"\w\pL]+)\)\)`)
	interpolationAnchoredRegex = regexp.MustCompile("\\A" + interpolationRegex.String() + "\\z")
)

type JSONOpt func(*json.Decoder) *json.Decoder

func UseNumber(dec *json.Decoder) *json.Decoder {
	dec.UseNumber()
	return dec
}

type String string

func (s String) Interpolate(resolver Resolver) (string, error) {
	var merr error
	interpolated := interpolationRegex.ReplaceAllStringFunc(string(s), func(name string) string {
		v, err := parseVar(name)
		if err != nil {
			merr = multierror.Append(merr, err)
			return name
		}
		var val interface{}
		if err := v.InterpolateInto(resolver, &val, UseNumber); err != nil {
			merr = multierror.Append(merr, err)
			return name
		}

		if val == nil {
			return "null"
		}

		switch val := val.(type) {
		case string, bool, json.Number:
			return fmt.Sprint(val)
		default:
			merr = multierror.Append(merr, InvalidInterpolationError{Name: name, Value: val})
			return name
		}
	})

	return interpolated, merr
}

func parseVar(name string) (Var, error) {
	name = strings.TrimPrefix(name, "((")
	name = strings.TrimSuffix(name, "))")

	ref, err := ParseReference(name)
	if err != nil {
		return Var{}, err
	}

	return Var(ref), nil
}

type Var Reference

func (v Var) isSet() bool {
	return v.Path != ""
}

func (v *Var) UnmarshalJSON(data []byte) error {
	var dst string
	if err := json.Unmarshal(data, &dst); err != nil {
		return err
	}
	if !interpolationAnchoredRegex.MatchString(dst) {
		return InvalidVarError{Name: dst}
	}
	ref, err := parseVar(dst)
	if err != nil {
		return err
	}
	*v = ref
	return nil
}

func (v Var) MarshalJSON() ([]byte, error) {
	return json.Marshal("((" + Reference(v).String() + "))")
}

func (v Var) InterpolateInto(resolver Resolver, dst interface{}, opts ...JSONOpt) error {
	val, err := resolver.Resolve(Reference(v))
	if err != nil {
		return err
	}
	payload, err := json.Marshal(val)
	if err != nil {
		return err
	}
	dec := decoder(payload, opts...)
	if err := dec.Decode(dst); err != nil {
		return err
	}
	return nil
}

func decoder(data []byte, opts ...JSONOpt) *json.Decoder {
	dec := json.NewDecoder(bytes.NewReader(data))
	for _, opt := range opts {
		dec = opt(dec)
	}
	return dec
}

type Any interface{}

func Interpolate(v Any, resolver Resolver) (interface{}, error) {
	payload, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var node interface{}
	if err := json.Unmarshal(payload, &node); err != nil {
		return nil, err
	}
	return interpolate(node, resolver)
}

func interpolate(node Any, resolver Resolver) (interface{}, error) {
	switch typedNode := node.(type) {
	case map[string]interface{}:
		for k, v := range typedNode {
			evaluatedKey, err := String(k).Interpolate(resolver)
			if err != nil {
				return nil, err
			}

			evaluatedValue, err := interpolate(v, resolver)
			if err != nil {
				return nil, err
			}

			typedNode[evaluatedKey] = evaluatedValue
			if k != evaluatedKey {
				delete(typedNode, k)
			}
		}
		return typedNode, nil

	case []interface{}:
		for i, x := range typedNode {
			var err error
			typedNode[i], err = interpolate(x, resolver)
			if err != nil {
				return nil, err
			}
		}
		return typedNode, nil

	case string:
		if interpolationAnchoredRegex.MatchString(typedNode) {
			v, err := parseVar(typedNode)
			if err != nil {
				return nil, err
			}
			var dst interface{}
			err = v.InterpolateInto(resolver, &dst)
			if err != nil {
				return nil, err
			}
			return dst, nil
		}
		return String(typedNode).Interpolate(resolver)
	}

	return node, nil
}

func InterpolateInto(a Any, resolver Resolver, dst interface{}, opts ...JSONOpt) error {
	val, err := Interpolate(a, resolver)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(val)
	if err != nil {
		return err
	}
	dec := decoder(payload, opts...)
	if err := dec.Decode(dst); err != nil {
		return err
	}
	return nil
}
