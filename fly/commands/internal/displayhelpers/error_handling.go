package displayhelpers

import (
	"fmt"
	"os"

	"github.com/concourse/fly/ui"
)

func PrintDeprecationWarningHeader() {
	printColorFunc := ui.ErroredColor.SprintFunc()
	fmt.Fprintf(ui.Stderr, "%s\n", printColorFunc("DEPRECATION WARNING:"))
}

func PrintWarningHeader() {
	printColorFunc := ui.BlinkingErrorColor.SprintFunc()
	fmt.Fprintf(ui.Stderr, "%s\n", printColorFunc("WARNING:"))
}

func Failf(message string, args ...interface{}) {
	fmt.Fprintf(ui.Stderr, message+"\n", args...)
	os.Exit(1)
}

func FailWithErrorf(message string, err error, args ...interface{}) {
	templatedMessage := fmt.Sprintf(message, args...)
	Failf("%s: %s", templatedMessage, err.Error())
}
