package flyerrors

import (
	"fmt"

	"github.com/fatih/color"
)

type ErrUnauthorized struct {
	error
}

func (e ErrUnauthorized) Error() string {
	line1 := "not authorized \nrun the following to log in:\n"
	line2 := "    " + color.New(color.Bold).SprintfFunc()("fly -t (alias) login -c (target url)\n")
	line3 := "type " + color.New(color.Bold).SprintFunc()("fly login -h") + " for more info"

	return fmt.Sprintf("%s %s %s", line1, line2, line3)
}
