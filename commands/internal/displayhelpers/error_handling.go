package displayhelpers

import (
	"fmt"
	"os"
)

func Failf(message string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, message+"\n", args...)
	os.Exit(1)
}

func FailWithErrorf(message string, err error, args ...interface{}) {
	templatedMessage := fmt.Sprintf(message, args...)
	Failf("%s: %s", templatedMessage, err.Error())
}
