package commands

import "github.com/jessevdk/go-flags"

type GlobalOptions struct {
	Target   string `short:"t" long:"target" description:"concourse API endpoint" default:"http://192.168.100.4:8080"`
	Insecure bool   `short:"k" long:"insecure" description:"skip SSL verification"`
}

var globalOptions GlobalOptions

var Parser = flags.NewParser(&globalOptions, flags.HelpFlag|flags.PassDoubleDash)
