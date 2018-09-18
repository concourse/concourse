package commands

import "errors"

var ErrShowHelpMessage = errors.New("help command invoked")

type HelpCommand struct{}

func (command *HelpCommand) Execute(args []string) error {
	return ErrShowHelpMessage
}
