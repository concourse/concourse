package util

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
)

var PanicSink io.Writer = os.Stderr

func DumpPanic(recovered interface{}, msg string, args ...interface{}) error {
	if recovered == nil {
		return nil
	}

	err := fmt.Errorf("panic in %s: %v", fmt.Sprintf(msg, args...), recovered)

	fmt.Fprintf(PanicSink, "%s\n %s\n", err.Error(), string(debug.Stack()))
	return err
}
