package hijackhelpers

import (
	"strings"

	"github.com/concourse/atc"
)

type ContainerSorter []atc.Container

func (sorter ContainerSorter) Len() int {
	return len(sorter)
}

func (sorter ContainerSorter) Swap(i, j int) {
	sorter[i], sorter[j] = sorter[j], sorter[i]
}

func (sorter ContainerSorter) Less(i, j int) bool {
	switch {
	case sorter[i].BuildID < sorter[j].BuildID:
		return true
	case sorter[i].BuildID > sorter[j].BuildID:
		return false
	case strings.Compare(sorter[i].ResourceName, sorter[j].ResourceName) == -1:
		return true
	case strings.Compare(sorter[i].ResourceName, sorter[j].ResourceName) == 1:
		return false
	case strings.Compare(sorter[i].StepName, sorter[j].StepName) == -1:
		return true
	case strings.Compare(sorter[i].StepName, sorter[j].StepName) == 1:
		return false
	case strings.Compare(sorter[i].Type, sorter[j].Type) == -1:
		return true
	default:
		return false
	}
}
