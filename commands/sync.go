package commands

import (
	"fmt"
	"runtime"

	"github.com/inconshreveable/go-update"

	"github.com/concourse/fly/internal/displayhelpers"
	"github.com/concourse/fly/rc"
)

type SyncCommand struct{}

func (command *SyncCommand) Execute(args []string) error {
	client, err := rc.TargetClient(Fly.Target)
	if err != nil {
		return err
	}

	body, err := client.GetCLIReader(runtime.GOARCH, runtime.GOOS)
	if err != nil {
		return err
	}

	fmt.Printf("downloading fly from %s... ", client.URL())

	err = update.Apply(body, update.Options{})
	if err != nil {
		displayhelpers.Failf("update failed: %s", err)
	}

	fmt.Println("update successful!")
	return nil
}
