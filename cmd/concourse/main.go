package main

import (
	"fmt"
	"os"

	flags "github.com/jessevdk/go-flags"
	"github.com/vito/twentythousandtonnesofcrudeoil"

	_ "github.com/concourse/atc/auth/genericoauth"
	_ "github.com/concourse/atc/auth/github"
	"github.com/concourse/atc/auth/provider"
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

	groups := parser.Command.Find("web").Groups()
	var authGroup *flags.Group

	for _, group := range groups {
		if group.ShortDescription == "Authentication" {
			authGroup = group
			break
		}
	}

	authConfigs := make(provider.AuthConfigs)

	for name, p := range provider.GetProviders() {
		authConfigs[name] = p.AddAuthGroup(authGroup)
	}

	twentythousandtonnesofcrudeoil.TheEnvironmentIsPerfectlySafe(parser, "CONCOURSE_")

	_, err := parser.Parse()

	cmd.Web.ProviderAuth = authConfigs

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
