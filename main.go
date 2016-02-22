package main

import (
	"fmt"
	"net"
	"os"
	"regexp"

	"github.com/concourse/fly/commands"
	"github.com/concourse/fly/rc"
	"github.com/concourse/go-concourse/concourse"
	"github.com/fatih/color"
	"github.com/jessevdk/go-flags"
)

func main() {
	parser := flags.NewParser(&commands.Fly, flags.HelpFlag|flags.PassDoubleDash)
	parser.NamespaceDelimiter = "-"

	embolden := color.New(color.Bold).SprintfFunc()

	_, err := parser.Parse()
	if err != nil {
		if err == concourse.ErrUnauthorized {
			fmt.Fprintln(os.Stderr, "not authorized. run the following to log in:")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "    "+embolden("fly -t %s login", commands.Fly.Target))
			fmt.Fprintln(os.Stderr, "")
		} else if err == rc.ErrNoTargetSpecified {
			fmt.Fprintln(os.Stderr, "no target specified. specify the target with "+embolden("-t")+" or log in like so:")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "    "+embolden("fly -t (alias) login -c (concourse url)"))
			fmt.Fprintln(os.Stderr, "")
		} else if netErr, ok := err.(net.Error); ok {
			fmt.Fprintf(os.Stderr, "could not reach the Concourse server called %s:\n", embolden("%s", commands.Fly.Target))

			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "    "+embolden("%s", netErr))
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
