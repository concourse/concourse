package ui

import (
	"sort"
	"strings"

	"github.com/concourse/concourse/atc"
)

func PresentVersion(version atc.Version) string {
	pairs := []string{}
	for k, v := range version {
		pairs = append(pairs, k+":"+v)
	}

	// consistent ordering
	sort.Strings(pairs)

	return strings.Join(pairs, ",")
}
