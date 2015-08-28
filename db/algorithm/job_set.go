package algorithm

type JobSet map[int]struct{}

func (set JobSet) Contains(jobID int) bool {
	_, found := set[jobID]
	return found
}

func (set JobSet) Union(otherSet JobSet) JobSet {
	newSet := JobSet{}

	for jobID, _ := range set {
		newSet[jobID] = struct{}{}
	}

	for jobID, _ := range otherSet {
		newSet[jobID] = struct{}{}
	}

	return newSet
}

func (set JobSet) Intersect(otherSet JobSet) JobSet {
	result := JobSet{}

	for key, val := range set {
		_, found := otherSet[key]
		if found {
			result[key] = val
		}
	}

	return result
}

func (set JobSet) Equal(otherSet JobSet) bool {
	if len(set) != len(otherSet) {
		return false
	}

	for x, _ := range set {
		if !otherSet.Contains(x) {
			return false
		}
	}

	return true
}
