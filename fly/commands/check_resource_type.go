package commands

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"
)

type CheckResourceTypeCommand struct {
	ResourceType flaghelpers.ResourceFlag `short:"r" long:"resource-type" required:"true" value-name:"PIPELINE/RESOURCE-TYPE" description:"Name of a resource-type to check"`
	Version      *atc.Version             `short:"f" long:"from"                     value-name:"VERSION"           description:"Version of the resource type to check from, e.g. digest:sha256@..."`
	Watch        bool                     `short:"w" long:"watch"                     value-name:"WATCH"           description:"Watch for the status of the check to succeed/fail"`
}

func (command *CheckResourceTypeCommand) Execute(args []string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	var version atc.Version
	if command.Version != nil {
		version = *command.Version
	}

	check, found, err := target.Team().CheckResourceType(command.ResourceType.PipelineName, command.ResourceType.ResourceName, version)
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("pipeline '%s' or resource-type '%s' not found\n", command.ResourceType.PipelineName, command.ResourceType.ResourceName)
	}

	var checkID = strconv.Itoa(check.ID)

	if command.Watch {
		for check.Status == "started" {
			time.Sleep(time.Second)

			check, found, err = target.Client().Check(checkID)
			if err != nil {
				return err
			}

			if !found {
				return fmt.Errorf("check '%s' not found\n", checkID)
			}
		}
	}

	table := ui.Table{
		Headers: ui.TableRow{
			{Contents: "id", Color: color.New(color.Bold)},
			{Contents: "status", Color: color.New(color.Bold)},
			{Contents: "check_error", Color: color.New(color.Bold)},
		},
	}

	table.Data = append(table.Data, []ui.TableCell{
		{Contents: checkID},
		{Contents: check.Status},
		{Contents: check.CheckError},
	})

	if err = table.Render(os.Stdout, Fly.PrintTableHeaders); err != nil {
		return err
	}

	if check.Status == "errored" {
		os.Exit(1)
	}

	return nil
}
