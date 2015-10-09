// +build windows

package commands

import "errors"

type HijackCommand struct{}

var hijackCommand HijackCommand

func init() {
	hijack, err := Parser.AddCommand(
		"hijack",
		"Execute an interactive command in a build's container",
		"",
		&hijackCommand,
	)
	if err != nil {
		panic(err)
	}

	hijack.Aliases = []string{"intercept", "i"}
}

func (command *HijackCommand) Execute(args []string) error {
	return errors.New("command not supported on windows!")
}
