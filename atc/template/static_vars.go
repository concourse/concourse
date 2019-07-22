package template

import (
	"strings"
)

type StaticVariables map[string]interface{}

var _ Variables = StaticVariables{}

func (v StaticVariables) Get(varDef VariableDefinition) (interface{}, bool, error) {
	val, found := v.processed()[varDef.Name]
	return val, found, nil
}

func (v StaticVariables) List() ([]VariableDefinition, error) {
	var defs []VariableDefinition

	for name, _ := range v.processed() {
		defs = append(defs, VariableDefinition{Name: name})
	}

	return defs, nil
}

func (v StaticVariables) processed() map[string]interface{} {
	processed := map[interface{}]interface{}{}

	for name, val := range v {
		pieces := strings.Split(name, ".")
		if len(pieces) == 1 {
			processed[name] = val
		} else {
			mapRef := processed

			for _, p := range pieces[0 : len(pieces)-1] {
				if _, found := processed[p]; !found {
					mapRef[p] = map[interface{}]interface{}{}
				}
				mapRef = mapRef[p].(map[interface{}]interface{})
			}

			mapRef[pieces[len(pieces)-1]] = val
		}
	}

	processedTyped := map[string]interface{}{}

	for k, v := range processed {
		processedTyped[k.(string)] = v
	}

	return processedTyped
}
