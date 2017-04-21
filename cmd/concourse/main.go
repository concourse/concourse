package main

import (
	"fmt"
	"os"

	"github.com/concourse/atc/auth/provider"
	flags "github.com/jessevdk/go-flags"
	"github.com/vito/twentythousandtonnesofcrudeoil"

	_ "github.com/concourse/atc/auth/genericoauth"
	_ "github.com/concourse/atc/auth/github"
	_ "github.com/concourse/atc/auth/uaa"
)

// overridden via linker flags
var Version = "0.0.0-dev"

func main() {
	var cmd ConcourseCommand

	cmd.Version = func() {
		fmt.Println(Version)
		os.Exit(0)
	}

	parser := flags.NewParser(&cmd, flags.HelpFlag|flags.PassDoubleDash)
	parser.NamespaceDelimiter = "-"

	cmd.lessenRequirements(parser)

	twentythousandtonnesofcrudeoil.TheEnvironmentIsPerfectlySafe(parser, "CONCOURSE_")

	authConfigs := make(provider.AuthConfigs)

	for name, p := range provider.GetProviders() {
		authGroup := p.AuthGroup()

		group, err := parser.Command.Group.AddGroup(authGroup.Name(), "", authGroup.AuthConfig())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		group.Namespace = authGroup.Namespace()

		authConfigs[name] = authGroup.AuthConfig()
	}

	_, err := parser.Parse()

	cmd.Web.ProviderAuth = authConfigs

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
