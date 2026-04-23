package commands

import (
	"errors"
	"fmt"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/rc"
)

type UnpauseComponentCommand struct {
	All               bool     `long:"all" short:"a" description:"Unpauses all components"`
	Name              []string `long:"name" short:"n" description:"Name of the component(s) to unpause. Can specify multiple times to unpause multiple components"`
	RuntimeComponents bool     `long:"runtime-components" description:"Unpauses all components related to starting and running pipelines"`
	GCComponents      bool     `long:"gc-components" description:"Unpauses all components related to garbage collection of data in the database, and artifacts (container, volumes) on Workers."`
}

func (command *UnpauseComponentCommand) Execute(args []string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	if command.All {
		err = target.Client().UnpauseAllComponents()
		if err != nil {
			return err
		}
		fmt.Println("all components unpaused")
		return nil
	}

	if command.RuntimeComponents {
		command.Name = append(command.Name, atc.ComponentsRuntime[:]...)
	}

	if command.GCComponents {
		command.Name = append(command.Name, atc.ComponentsGarbageCollection[:]...)
	}

	if len(command.Name) == 0 {
		return errors.New("--name or --all must be provided")
	}

	for _, name := range command.Name {
		err = target.Client().UnpauseComponent(name)
		if err != nil {
			return err
		}
		fmt.Printf("unpaused '%s'\n", name)
	}

	return nil
}
