package backend

import (
	"fmt"

	"code.cloudfoundry.org/garden"
)

// propertiesToFilterList converts a set of garden properties to a list of
// filters as expected by containerd.
//
func propertiesToFilterList(properties garden.Properties) (filters []string, err error) {
	filters = make([]string, len(properties))

	idx := 0
	for k, v := range properties {
		if k == "" || v == "" {
			err = fmt.Errorf("key or value must not be empty")
			return
		}

		filters[idx] = k + "=" + v
		idx++
	}

	return
}
