package commands

import (
	"fmt"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/rc"
)

type SetWallCommand struct {
	Message string        `short:"m" long:"message" required:"true" description:"Message to broadcast. Supports emojis and links."`
	TTL     time.Duration `long:"ttl" required:"false" description:"Time-to-live for the message (e.g. 1h30m). Zero values will result in a message with no expiration."`
}

func (command *SetWallCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	wall := atc.Wall{
		Message: command.Message,
		TTL:     command.TTL,
	}

	err = target.Client().SetWall(wall)
	if err != nil {
		return fmt.Errorf("failed to set wall message: %w", err)
	}

	fmt.Println("Wall message set successfully")
	if command.TTL == 0 {
		fmt.Println("No expiration set")
	} else {
		fmt.Printf("Message will expire in %v\n", command.TTL)
	}

	return nil
}
