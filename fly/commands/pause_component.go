package commands

import (
	"errors"
	"fmt"

	"github.com/concourse/concourse/fly/rc"
)

type PauseComponentCommand struct {
	All  bool     `long:"all" short:"a" description:"Pauses all components"`
	Name []string `long:"name" short:"n" description:"Name of the component(s) to unpause. Can specify multiple times to pause multiple components"`
}

func (command *PauseComponentCommand) Execute(args []string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	if command.All {
		err = target.Client().PauseAllComponents()
		if err != nil {
			return err
		}
		fmt.Println("all components paused")
		return nil
	}

	if len(command.Name) == 0 {
		return errors.New("--name or --all must be provided")
	}

	for _, name := range command.Name {
		err = target.Client().PauseComponent(name)
		if err != nil {
			return err
		}
		fmt.Printf("paused '%s'\n", name)
	}

	return nil
}
