package main

import (
	"fmt"
	"os"

	"github.com/concourse/concourse"
	"github.com/concourse/concourse/atc/atccmd"
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

	cmd.lessenRequirements(parser)

	cmd.Web.WireDynamicFlags(parser.Command.Find("web"))
	cmd.Quickstart.WebCommand.WireDynamicFlags(parser.Command.Find("quickstart"))

	twentythousandtonnesofcrudeoil.TheEnvironmentIsPerfectlySafe(parser, "CONCOURSE_")

	_, err := parser.Parse()
	if err != nil {
		if err == atccmd.HelpError {
			parser.WriteHelp(os.Stdout)
			os.Exit(1)
		} else {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
