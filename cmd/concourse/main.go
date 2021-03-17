package main

import (
	"fmt"
	"os"

	"github.com/concourse/concourse"
	flags "github.com/jessevdk/go-flags"
	"github.com/vito/twentythousandtonnesofcrudeoil"
)

func main() {
	var cmd ConcourseCommand

	cmd.Version = func() {
		fmt.Println(concourse.Version)
		os.Exit(0)
	}

	parser := flags.NewParser(&cmd, flags.HelpFlag|flags.PassDoubleDash)
	parser.NamespaceDelimiter = "-"

	cmd.LessenRequirements(parser)

	cmd.Web.WireDynamicFlags(parser.Command.Find("web"))
	cmd.Quickstart.WebCommand.WireDynamicFlags(parser.Command.Find("quickstart"))

	twentythousandtonnesofcrudeoil.TheEnvironmentIsPerfectlySafe(parser, "CONCOURSE_")

	_, err := parser.Parse()
	handleError(parser, err)
}

func handleError(helpParser *flags.Parser, err error) {
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			fmt.Println(err)
			os.Exit(0)
		} else {
			fmt.Fprintf(os.Stderr, "error: %s\n", err)
		}

		os.Exit(1)
	}
}
