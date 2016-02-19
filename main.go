package main

import (
	"fmt"
	"os"
	"regexp"

	"github.com/concourse/fly/commands"
	"github.com/concourse/go-concourse/concourse"
	"github.com/fatih/color"
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

			if commands.Fly.Target == "" {
				fmt.Fprintln(os.Stderr, "    "+color.New(color.Bold).SprintfFunc()("fly -t (alias) login -c %s", commands.Fly.Target))
			} else {
				fmt.Fprintln(os.Stderr, "    "+color.New(color.Bold).SprintfFunc()("fly -t %s login", commands.Fly.Target))
			}

			fmt.Fprintln(os.Stderr, "")
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
