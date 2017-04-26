package main

import (
	"fmt"
	"os"

	"github.com/concourse/atc/atccmd"
	"github.com/concourse/atc/auth/provider"
	"github.com/jessevdk/go-flags"

	_ "github.com/concourse/atc/auth/genericoauth"
	_ "github.com/concourse/atc/auth/github"
	_ "github.com/concourse/atc/auth/uaa"
)

func main() {
	cmd := &atccmd.ATCCommand{}

	parser := flags.NewParser(cmd, flags.Default)
	parser.NamespaceDelimiter = "-"

	authConfigs := make(provider.AuthConfigs)

	for name, p := range provider.GetProviders() {
		authConfigs[name] = p.AddAuthGroup(parser.Command.Group)
	}

	args, err := parser.Parse()
	if err != nil {
		os.Exit(1)
	}

	cmd.ProviderAuth = authConfigs
	err = cmd.Execute(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
