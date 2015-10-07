package commands

import (
	"fmt"

	"github.com/jessevdk/go-flags"
)

type targetPrinter struct {
	flags.Commander
}

func (command *targetPrinter) Execute(args []string) error {
	fmt.Println("currently targeting", globalOptions.Target)
	return command.Commander.Execute(args)
}
