package algorithm

import (
	"fmt"
	"sort"
	"strings"
)

type BuildSet map[int]struct{}

func (set BuildSet) Contains(buildID int) bool {
	_, found := set[buildID]
	return found
}

func (set BuildSet) Union(otherSet BuildSet) BuildSet {
	newSet := BuildSet{}

	for buildID := range set {
		newSet[buildID] = struct{}{}
	}

	for buildID := range otherSet {
		newSet[buildID] = struct{}{}
	}

	return newSet
}

func (set BuildSet) Intersect(otherSet BuildSet) BuildSet {
	result := BuildSet{}

	for key, val := range set {
		_, found := otherSet[key]
		if found {
			result[key] = val
		}
	}

	return result
}

func (set BuildSet) Overlaps(otherSet BuildSet) bool {
	check := set
	against := otherSet
	if len(check) > len(against) {
		check, against = against, check
	}

	for key := range check {
		_, found := against[key]
		if found {
			return true
		}
	}

	return false
}

func (set BuildSet) Equal(otherSet BuildSet) bool {
	if len(set) != len(otherSet) {
		return false
	}

	for x := range set {
		if !otherSet.Contains(x) {
			return false
		}
	}

	return true
}

func (set BuildSet) String() string {
	xs := []string{}
	for x := range set {
		xs = append(xs, fmt.Sprintf("%v", x))
	}

	sort.Strings(xs)

	return fmt.Sprintf("{%s}", strings.Join(xs, " "))
}
