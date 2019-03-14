package displayhelpers

import (
	"fmt"
	"github.com/concourse/concourse/go-concourse/concourse"
	"os"

	"github.com/concourse/concourse/fly/ui"
)

func PrintDeprecationWarningHeader() {
	printColorFunc := ui.ErroredColor.SprintFunc()
	fmt.Fprintf(ui.Stderr, "%s\n", printColorFunc("DEPRECATION WARNING:"))
}

func PrintWarningHeader() {
	printColorFunc := ui.BlinkingErrorColor.SprintFunc()
	fmt.Fprintf(ui.Stderr, "%s\n", printColorFunc("WARNING:"))
}

func ShowErrors(errorHeader string, errorMessages []string) {
	fmt.Fprintln(ui.Stderr, "")
	PrintWarningHeader()

	fmt.Fprintln(ui.Stderr, errorHeader+":")
	for _, errorMessage := range errorMessages {
		fmt.Fprintf(ui.Stderr, "  - %s\n", errorMessage)
	}

	fmt.Fprintln(ui.Stderr, "")
}

func ShowWarnings(warnings []concourse.ConfigWarning) {
	fmt.Fprintln(ui.Stderr, "")
	PrintDeprecationWarningHeader()

	for _, warning := range warnings {
		fmt.Fprintf(ui.Stderr, "  - %s\n", warning.Message)
	}

	fmt.Fprintln(ui.Stderr, "")
}

func Failf(message string, args ...interface{}) {
	fmt.Fprintf(ui.Stderr, message+"\n", args...)
	os.Exit(1)
}

func FailWithErrorf(message string, err error, args ...interface{}) {
	templatedMessage := fmt.Sprintf(message, args...)
	Failf("%s: %s", templatedMessage, err.Error())
}
