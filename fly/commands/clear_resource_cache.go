package commands

import (
	"fmt"

	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
)

type ClearResourceCacheCommand struct {
	Text string `short:"t" long:"text" default:"hello CLI" description:"Some text"`
}

func (command *ClearResourceCacheCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	fmt.Fprintf(ui.Stderr, "text: %s", command.Text)

	return nil
}
