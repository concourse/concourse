package commands

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/concourse/concourse/v8/atc"
	"github.com/concourse/concourse/v8/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/v8/fly/commands/internal/interaction"
	"github.com/concourse/concourse/v8/fly/rc"
	"github.com/concourse/concourse/v8/go-concourse/concourse"
)

type ClearResourceCacheCommand struct {
	Pipeline *flaghelpers.PipelineFlag `short:"p" long:"pipeline" description:"Pipeline whose resource caches to clear"`
	Resource flaghelpers.ResourceFlag  `short:"r" long:"resource" value-name:"PIPELINE/RESOURCE" description:"Name of a resource to clear cache"`
	Version  *atc.Version              `short:"v" long:"version"                  value-name:"VERSION"           description:"Version of the resource to check from, e.g. digest:sha256@..., in case a version is not specified the command will delete all the resource caches for that resource"`
	Team     flaghelpers.TeamFlag      `long:"team" description:"Name of the team to which the pipeline/resource belong, if different from the target default"`
}

func (command *ClearResourceCacheCommand) Execute(args []string) error {
	resourceSpecified := command.Resource.ResourceName != ""
	if command.Pipeline != nil && resourceSpecified {
		return errors.New("cannot specify both --pipeline and --resource")
	}
	if command.Pipeline == nil && !resourceSpecified {
		return errors.New("must specify either --pipeline or --resource")
	}
	if command.Pipeline != nil && command.Version != nil {
		return errors.New("--version requires --resource")
	}

	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	var team concourse.Team
	team, err = command.Team.LoadTeam(target)
	if err != nil {
		return err
	}

	if command.Pipeline != nil {
		return command.clearPipelineCaches(team)
	}

	var version atc.Version
	warningMsg := fmt.Sprintf("!!! this will remove the resource cache(s) for `%s/%s`",
		command.Resource.PipelineRef.String(), command.Resource.ResourceName)

	if command.Version != nil {
		version = *command.Version
		versionJson, err := json.Marshal(version)
		if err != nil {
			return err
		}
		warningMsg += fmt.Sprintf(" with version: `%s`", versionJson)
	}

	confirm, err := confirmResourceCacheClear(warningMsg)
	if err != nil || !confirm {
		return err
	}

	numRemoved, err := team.ClearResourceCache(command.Resource.PipelineRef, command.Resource.ResourceName, version)

	if err != nil {
		return err
	} else {
		fmt.Printf("%d caches removed\n", numRemoved)
		return nil
	}
}

func (command *ClearResourceCacheCommand) clearPipelineCaches(team concourse.Team) error {
	pipeline := command.Pipeline.Ref()
	resources, err := team.ListResources(pipeline)
	if err != nil {
		return err
	}
	if len(resources) == 0 {
		fmt.Println("no resources found")
		return nil
	}

	confirmed, err := confirmResourceCacheClear(fmt.Sprintf("!!! this will remove the resource cache(s) for all resources in pipeline '%s'", pipeline))
	if err != nil || !confirmed {
		fmt.Println("bailing out")
		return err
	}

	var total int64
	var errs []error
	for _, resource := range resources {
		removed, err := team.ClearResourceCache(pipeline, resource.Name, nil)
		if err != nil {
			errs = append(errs, fmt.Errorf("error clearing cache for '%s': %w", resource.Name, err))
			continue
		}
		total += removed
	}
	fmt.Printf("%d caches removed\n", total)
	return errors.Join(errs...)
}

func confirmResourceCacheClear(warning string) (bool, error) {
	fmt.Print(warning + "\n\n")
	confirmed, err := interaction.Confirm("are you sure?")
	if err != nil || !confirmed {
		fmt.Println("bailing out")
	}
	return confirmed, err
}
