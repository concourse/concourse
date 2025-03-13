package vars

type StaticVariables map[string]any

var _ Variables = StaticVariables{}

func (v StaticVariables) Get(ref Reference) (any, bool, error) {
	if ref.Source != "" {
		return nil, false, nil
	}

	val, found := v[ref.Path]
	if !found {
		return nil, false, nil
	}
	val, err := Traverse(val, ref.String(), ref.Fields)
	if err != nil {
		return nil, false, err
	}
	return val, true, nil
}

func (v StaticVariables) List() ([]Reference, error) {
	var refs []Reference

	for name, _ := range v {
		refs = append(refs, Reference{Path: name})
	}

	return refs, nil
}

func Traverse(val any, name string, fields []string) (any, error) {
	for _, seg := range fields {
		switch v := val.(type) {
		case map[any]any:
			var found bool
			val, found = v[seg]
			if !found {
				return nil, MissingFieldError{
					Name:  name,
					Field: seg,
				}
			}
		case map[string]any:
			var found bool
			val, found = v[seg]
			if !found {
				return nil, MissingFieldError{
					Name:  name,
					Field: seg,
				}
			}
		default:
			return nil, InvalidFieldError{
				Name:  name,
				Field: seg,
				Value: val,
			}
		}
	}
	return val, nil
}

func (v StaticVariables) Flatten() KVPairs {
	var flat KVPairs
	for k, vv := range v {
		flat = append(flat, flatten(k, nil, vv)...)
	}
	return flat
}

func flatten(path string, fields []string, value any) KVPairs {
	var flat KVPairs

	switch node := value.(type) {
	case map[string]any:
		for k, v := range node {
			flat = append(flat, flatten(path, append(fields, k), v)...)
		}
	case map[any]any:
		for k, v := range node {
			if str, ok := k.(string); ok {
				flat = append(flat, flatten(path, append(fields, str), v)...)
			}
		}
	default:
		flat = KVPairs{{
			Ref: Reference{
				Path:   path,
				Fields: fields,
			},
			Value: value,
		}}
	}

	return flat
}

type KVPair struct {
	Ref   Reference
	Value any
}

type KVPairs []KVPair

func (p KVPairs) Expand() StaticVariables {
	out := make(map[string]any, len(p))
	for _, pair := range p {
		upsert(out, pair.Ref.Path, pair.Ref.Fields, pair.Value)
	}
	return out
}

func upsert(out map[string]any, path string, fields []string, value any) {
	node, ok := out[path]
	if !ok {
		out[path] = constructValue(fields, value)
		return
	}
	nodeMap, ok := node.(map[string]any)
	if !ok {
		out[path] = constructValue(fields, value)
		return
	}
	if len(fields) == 0 {
		out[path] = value
		return
	}
	upsert(nodeMap, fields[0], fields[1:], value)
}

func constructValue(fields []string, value any) any {
	if len(fields) == 0 {
		return value
	}
	return constructValue(fields[:len(fields)-1], map[string]any{fields[len(fields)-1]: value})
}
