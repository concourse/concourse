package main

import (
	"fmt"
	"os"
	"regexp"

	"github.com/concourse/fly/commands"
	"github.com/concourse/fly/internal/displayhelpers"
	"github.com/jessevdk/go-flags"
)

func main() {
	parser := flags.NewParser(&commands.Fly, flags.HelpFlag|flags.PassDoubleDash)
	parser.NamespaceDelimiter = "-"

	_, err := parser.Parse()
	if err != nil {
		flyError := displayhelpers.TranslateErrors(err)
		fmt.Fprintln(os.Stderr, flyError.Error())

		os.Exit(1)
	}
}

func isURL(passedURL string) bool {
	matched, _ := regexp.MatchString("^http[s]?://", passedURL)
	return matched
}
