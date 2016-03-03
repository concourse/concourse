package displayhelpers

import (
	"fmt"
	"os"

	"github.com/concourse/fly/ui"
)

func Warn(message string) {
	printColorFunc := ui.ErroredColor.SprintFunc()
	fmt.Fprintf(os.Stderr, "%s\n", printColorFunc(message))
}

func Failf(message string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, message+"\n", args...)
	os.Exit(1)
}

func FailWithErrorf(message string, err error, args ...interface{}) {
	templatedMessage := fmt.Sprintf(message, args...)
	Failf("%s: %s", templatedMessage, err.Error())
}
