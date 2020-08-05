package rc

import (
	"sort"
	"strings"

	"github.com/jessevdk/go-flags"
)

type TargetName string

func (name *TargetName) UnmarshalFlag(value string) error {
	*name = TargetName(value)
	return nil
}

func (name *TargetName) Complete(match string) []flags.Completion {
	flyTargets, err := LoadTargets()
	if err != nil {
		return []flags.Completion{}
	}

	names := []string{}
	for name := range flyTargets {
		if strings.HasPrefix(string(name), match) {
			names = append(names, string(name))
		}
	}

	sort.Strings(names)

	comps := make([]flags.Completion, len(names))
	for idx, name := range names {
		comps[idx].Item = name
	}

	return comps
}
