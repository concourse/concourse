package vars

//go:generate counterfeiter . Variables

type Variables interface {
	Get(VariableDefinition) (interface{}, bool, error)
	List() ([]VariableDefinition, error)
}

type VariableReference struct {
	Name   string
	Source string
	Path   string
	Fields []string
}

type VariableDefinition struct {
	Ref     VariableReference
	Type    string
	Options interface{}
}
