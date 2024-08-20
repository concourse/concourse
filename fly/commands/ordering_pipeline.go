package commands

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/rc"
)

var ErrMissingPipelineName = errors.New("Need to specify at least one pipeline name")

type OrderPipelinesCommand struct {
	Alphabetical bool                 `short:"a"  long:"alphabetical" description:"Order all pipelines alphabetically"`
	Pipelines    []string             `short:"p" long:"pipeline" description:"Name of pipeline (can be specified multiple times to provide relative ordering)"`
	Team         flaghelpers.TeamFlag `long:"team" description:"Name of the team to which the pipelines belong, if different from the target default"`
}

func (command *OrderPipelinesCommand) Execute(args []string) error {
	if !command.Alphabetical && command.Pipelines == nil {
		displayhelpers.Failf("error: either --pipeline or --alphabetical are required")
	}

	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}
	teamName := command.Team.Name()
	if teamName == "" {
		teamName = target.Team().Name()
	}
	team, err := target.FindTeam(teamName)
	if err != nil {
		return err
	}
	var orderedNames []string
	if command.Alphabetical {
		seen := map[string]bool{}

		if err != nil {
			return err
		}
		ps, err := team.ListPipelines()
		if err != nil {
			return err
		}
		for _, p := range ps {
			if !seen[p.Name] {
				seen[p.Name] = true
				orderedNames = append(orderedNames, p.Name)
			}
		}
		sort.Strings(orderedNames)
	} else {
		for _, p := range command.Pipelines {
			if strings.ContainsRune(p, '/') {
				return fmt.Errorf("pipeline name %q cannot contain '/'", p)
			}
		}
		orderedNames = command.Pipelines
	}

	err = team.OrderingPipelines(orderedNames)
	if err != nil {
		displayhelpers.FailWithErrorf("failed to order pipelines", err)
	}

	fmt.Printf("ordered pipelines \n")
	for _, p := range orderedNames {
		fmt.Printf("  - %s \n", p)
	}

	return nil
}
