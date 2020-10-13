package vars

//go:generate counterfeiter . Variables

type Variables interface {
	Get(Reference) (interface{}, bool, error)
	List() ([]Reference, error)
}

type Reference struct {
	Name   string
	Source string
	Path   string
	Fields []string
}
