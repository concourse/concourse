package commands

import "github.com/jessevdk/go-flags"

type GlobalOptions struct {
	Target string `short:"t" long:"target" description:"Concourse target name or URL" default:"http://192.168.50.4:8080"`
}

var globalOptions GlobalOptions

var Parser = flags.NewParser(&globalOptions, flags.HelpFlag|flags.PassDoubleDash)
