package commands

import (
	"fmt"

	"github.com/concourse/concourse/fly/rc"
)

type ClearWallCommand struct{}

func (command *ClearWallCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	err = target.Client().ClearWall()
	if err != nil {
		return fmt.Errorf("failed to clear wall message: %w", err)
	}

	fmt.Println("Wall message cleared successfully")
	return nil
}
