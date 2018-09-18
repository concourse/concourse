package algorithm

type Constraints []Constraint
type Constraint func(Version) bool

func (cs Constraints) Check(v Version) bool {
	for _, c := range cs {
		if !c(v) {
			return false
		}
	}

	return true
}

func (cs Constraints) And(constraint Constraint) Constraints {
	ncs := make([]Constraint, len(cs)+1)
	copy(ncs, cs)
	ncs[len(cs)] = constraint
	return ncs
}
