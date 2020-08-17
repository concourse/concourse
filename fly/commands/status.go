package commands

import (
	"fmt"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/rc"
)

type StatusCommand struct{}

func (c *StatusCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	tToken := target.Token()

	if tToken == nil || tToken.Value == "" {
		displayhelpers.Failf("logged out")
		return nil
	}

	_, err = target.Client().UserInfo()
	if err != nil {
		displayhelpers.FailWithErrorf("please login again.\n\ntoken validation failed with error", err)
		return nil
	}

	fmt.Println("logged in successfully")
	return nil
}
