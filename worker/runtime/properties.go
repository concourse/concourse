package runtime

import (
	"fmt"

	"code.cloudfoundry.org/garden"
)

// propertiesToFilterList converts a set of garden properties to a list of
// filters as expected by containerd.
//
// containerd filters are in the form of
//
//           <what>.<field><operator><value>
//
// which, in our very specific case of properties, means
//
//           labels.foo==bar
//           |      |  | value
//           |      |  equality
//           |      key
//           what
//
func propertiesToFilterList(properties garden.Properties) (filters []string, err error) {
	filters = make([]string, len(properties))

	idx := 0
	for k, v := range properties {
		if k == "" || v == "" {
			err = fmt.Errorf("key or value must not be empty")
			return
		}

		filters[idx] = "labels." + k + "==" + v
		idx++
	}

	return
}
