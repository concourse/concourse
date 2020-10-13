package vars

//go:generate counterfeiter . Variables

type Variables interface {
	Get(VariableDefinition) (interface{}, bool, error)
	List() ([]VariableDefinition, error)
}

type Reference struct {
	Name   string
	Source string
	Path   string
	Fields []string
}

type VariableDefinition struct {
	Ref     Reference
	Type    string
	Options interface{}
}
