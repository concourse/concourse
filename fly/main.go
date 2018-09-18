package main

import (
	"fmt"
	"net"
	"os"

	"github.com/concourse/fly/commands"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/concourse/go-concourse/concourse"
	"github.com/jessevdk/go-flags"
)

func main() {
	parser := flags.NewParser(&commands.Fly, flags.HelpFlag|flags.PassDoubleDash)
	parser.NamespaceDelimiter = "-"

	helpParser := flags.NewParser(&commands.Fly, flags.HelpFlag)
	helpParser.NamespaceDelimiter = "-"

	commands.WireTeamConnectors(parser.Find("set-team"))

	_, err := parser.Parse()
	err = loginAndRetry(parser, err)
	handleError(helpParser, err)
}

func loginAndRetry(parser *flags.Parser, err error) error {
	_, stdoutIsTTY := ui.ForTTY(os.Stdout)
	_, stdinIsTTY := ui.ForTTY(os.Stdin)
	if err == concourse.ErrUnauthorized && stdoutIsTTY && stdinIsTTY {
		fmt.Fprintln(ui.Stderr, "could not find a valid token.")

		login := &commands.LoginCommand{}
		err = login.Execute([]string{})

		if err == nil {
			_, err = parser.Parse()
		}
	}
	return err
}

func handleError(helpParser *flags.Parser, err error) {
	if err != nil {
		if err == concourse.ErrUnauthorized {
			fmt.Fprintln(ui.Stderr, "not authorized. run the following to log in:")
			fmt.Fprintln(ui.Stderr, "")
			fmt.Fprintln(ui.Stderr, "    "+ui.Embolden("fly -t %s login", commands.Fly.Target))
			fmt.Fprintln(ui.Stderr, "")
		} else if err == rc.ErrNoTargetSpecified {
			fmt.Fprintln(ui.Stderr, "no target specified. specify the target with "+ui.Embolden("-t")+" or log in like so:")
			fmt.Fprintln(ui.Stderr, "")
			fmt.Fprintln(ui.Stderr, "    "+ui.Embolden("fly -t (alias) login -c (concourse url)"))
			fmt.Fprintln(ui.Stderr, "")
		} else if versionErr, ok := err.(rc.ErrVersionMismatch); ok {
			fmt.Fprintln(ui.Stderr, versionErr.Error())
			fmt.Fprintln(ui.Stderr, ui.WarningColor("cowardly refusing to run due to significant version discrepancy"))
		} else if netErr, ok := err.(net.Error); ok {
			fmt.Fprintf(ui.Stderr, "could not reach the Concourse server called %s:\n", ui.Embolden("%s", commands.Fly.Target))
			fmt.Fprintln(ui.Stderr, "")
			fmt.Fprintln(ui.Stderr, "    "+ui.Embolden("%s", netErr))
			fmt.Fprintln(ui.Stderr, "")
			fmt.Fprintln(ui.Stderr, "is the targeted Concourse running? better go catch it lol")
		} else if err == commands.ErrShowHelpMessage {
			helpParser.ParseArgs([]string{"-h"})
			helpParser.WriteHelp(os.Stdout)
			os.Exit(0)
		} else if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrCommandRequired {
			helpParser.ParseArgs([]string{"-h"})
			helpParser.WriteHelp(os.Stdout)
			os.Exit(0)
		} else {
			fmt.Fprintf(ui.Stderr, "error: %s\n", err)
		}

		os.Exit(1)
	}
}
