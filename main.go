package main

import (
	"fmt"
	"net"
	"os"
	"regexp"

	"github.com/concourse/fly/commands"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/concourse/go-concourse/concourse"
	"github.com/jessevdk/go-flags"
)

func main() {
	parser := flags.NewParser(&commands.Fly, flags.HelpFlag|flags.PassDoubleDash)
	parser.NamespaceDelimiter = "-"

	_, err := parser.Parse()
	if err != nil {
		if err == concourse.ErrUnauthorized {
			fmt.Fprintln(os.Stderr, "not authorized. run the following to log in:")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "    "+ui.Embolden("fly -t %s login", commands.Fly.Target))
			fmt.Fprintln(os.Stderr, "")
		} else if err == rc.ErrNoTargetSpecified {
			fmt.Fprintln(os.Stderr, "no target specified. specify the target with "+ui.Embolden("-t")+" or log in like so:")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "    "+ui.Embolden("fly -t (alias) login -c (concourse url)"))
			fmt.Fprintln(os.Stderr, "")
		} else if err == rc.ErrVersionMismatch {
			fmt.Fprintln(os.Stderr, "fly version is out of sync with the target. run the following command to re-sync it:")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "    "+ui.Embolden("fly -t (alias) sync"))
			fmt.Fprintln(os.Stderr, "")
		} else if netErr, ok := err.(net.Error); ok {
			fmt.Fprintf(os.Stderr, "could not reach the Concourse server called %s:\n", ui.Embolden("%s", commands.Fly.Target))

			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "    "+ui.Embolden("%s", netErr))
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "is the targeted Concourse running? better go catch it lol")
		} else {
			fmt.Fprintf(os.Stderr, "error: %s\n", err)
		}

		os.Exit(1)
	}
}

func isURL(passedURL string) bool {
	matched, _ := regexp.MatchString("^http[s]?://", passedURL)
	return matched
}
