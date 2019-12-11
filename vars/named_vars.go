package vars

import (
	"fmt"
	"strings"
)

type NamedVariables struct {
	varss map[string]Variables
}

func NewNamedVariables(varss map[string]Variables) NamedVariables {
	return NamedVariables{varss}
}

// Get checks var_source if presents, then forward var to underlying secret manager.
// A `varDef.Name` with a var_source looks like "myvault:foo", where "myvault" is
// the var_source name, and "foo" is the real var name that should be forwarded
// to the underlying secret manager.
func (m NamedVariables) Get(varDef VariableDefinition) (interface{}, bool, error) {
	var sourceName, varName string

	parts := strings.Split(varDef.Name, ":")
	if len(parts) == 1 {
		// No source name, then no need to query named vars.
		return nil, false, nil
	} else if len(parts) == 2 {
		sourceName = parts[0]
		varName = parts[1]
	} else {
		return nil, false, fmt.Errorf("invalid var: %s", varDef.Name)
	}

	if vars, ok := m.varss[sourceName]; ok {
		return vars.Get(VariableDefinition{Name: varName})
	}

	return nil, false, nil
}

func (m NamedVariables) List() ([]VariableDefinition, error) {
	var allDefs []VariableDefinition

	for _, vars := range m.varss {
		defs, err := vars.List()
		if err != nil {
			return nil, err
		}

		allDefs = append(allDefs, defs...)
	}

	return allDefs, nil
}
