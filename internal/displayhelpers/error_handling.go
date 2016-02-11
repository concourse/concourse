package displayhelpers

import (
	"fmt"
	"os"

	"github.com/concourse/fly/internal/flyerrors"
	"github.com/concourse/go-concourse/concourse"
)

func Failf(message string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, message+"\n", args...)
	os.Exit(1)
}

func TranslateErrors(err error) error {
	if concourse.ErrUnauthorized == err {
		return flyerrors.ErrUnauthorized{}
	} else {
		return err
	}
}

func FailWithErrorf(message string, err error, args ...interface{}) {
	templatedMessage := fmt.Sprintf(message, args...)

	flyError := TranslateErrors(err)

	Failf("%s: %s", templatedMessage, flyError.Error())
}
