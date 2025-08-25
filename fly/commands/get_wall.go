package commands

import (
	"fmt"

	"github.com/concourse/concourse/fly/rc"
)

type GetWallCommand struct{}

func (command *GetWallCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	wall, err := target.Client().GetWall()
	if err != nil {
		return fmt.Errorf("failed to get wall message: %w", err)
	}

	if wall.Message == "" {
		fmt.Println("No wall message is currently set")
		return nil
	}

	fmt.Printf("Wall Message: %s\n", wall.Message)
	fmt.Printf("Expires in: %v\n", wall.TTL)

	return nil
}
