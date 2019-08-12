package flaghelpers

import (
	"strings"

	"github.com/concourse/concourse/fly/rc"
	"github.com/jessevdk/go-flags"
)

type TeamFlag string

func (flag *TeamFlag) Complete(match string) []flags.Completion {
	fly := parseFlags()

	target, err := rc.LoadTarget(fly.Target, false)
	if err != nil {
		return []flags.Completion{}
	}

	teams, err := target.Client().ListTeams()
	if err != nil {
		return []flags.Completion{}
	}

	comps := []flags.Completion{}
	for _, team := range teams {
		if strings.HasPrefix(team.Name, match) {
			comps = append(comps, flags.Completion{Item: team.Name})
		}
	}

	return comps
}

func (flag TeamFlag) Name() string {
	return string(flag)
}
