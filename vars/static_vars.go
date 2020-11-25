package vars

type StaticVariables map[string]interface{}

var _ Variables = StaticVariables{}

func (v StaticVariables) Get(varDef VariableDefinition) (interface{}, bool, error) {
	val, found := v[varDef.Ref.Path]
	return val, found, nil
}

func (v StaticVariables) List() ([]VariableDefinition, error) {
	var defs []VariableDefinition

	for name, _ := range v {
		defs = append(defs, VariableDefinition{Ref: VariableReference{Path: name}})
	}

	return defs, nil
}

func (v StaticVariables) Flatten() KVPairs {
	var flat KVPairs
	for k, vv := range v {
		flat = append(flat, flatten(k, nil, vv)...)
	}
	return flat
}

func flatten(path string, fields []string, value interface{}) KVPairs {
	var flat KVPairs

	switch node := value.(type) {
	case map[string]interface{}:
		for k, v := range node {
			flat = append(flat, flatten(path, append(fields, k), v)...)
		}
	case map[interface{}]interface{}:
		for k, v := range node {
			if str, ok := k.(string); ok {
				flat = append(flat, flatten(path, append(fields, str), v)...)
			}
		}
	default:
		flat = KVPairs{{
			Ref: VariableReference{
				Path:   path,
				Fields: fields,
			},
			Value: value,
		}}
	}

	return flat
}

type KVPair struct {
	Ref   VariableReference
	Value interface{}
}

type KVPairs []KVPair

func (f KVPairs) Expand() StaticVariables {
	out := map[string]interface{}{}
	for _, pair := range f {
		upsert(out, pair.Ref.Path, pair.Ref.Fields, pair.Value)
	}
	return out
}

func upsert(out map[string]interface{}, path string, fields []string, value interface{}) {
	node, ok := out[path]
	if !ok {
		out[path] = constructValue(fields, value)
		return
	}
	nodeMap, ok := node.(map[string]interface{})
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

func constructValue(fields []string, value interface{}) interface{} {
	if len(fields) == 0 {
		return value
	}
	return constructValue(fields[:len(fields)-1], map[string]interface{}{fields[len(fields)-1]: value})
}
