package vars

type Resolver interface {
	Resolve(Reference) (interface{}, error)
}

type ResolverFunc func(Reference) (interface{}, error)

func (r ResolverFunc) Resolve(ref Reference) (interface{}, error) {
	return r(ref)
}

type resolver struct {
	vars Variables
}

func NewResolver(v Variables) Resolver {
	return resolver{vars: v}
}

func (r resolver) Resolve(ref Reference) (interface{}, error) {
	val, found, err := r.vars.Get(ref)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, UndefinedVarsError{Vars: []string{ref.String()}}
	}
	return val, nil
}
