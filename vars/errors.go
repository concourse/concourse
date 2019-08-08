package vars

import (
	"fmt"
	"strings"
)

type UndefinedVarsError struct {
	Vars []string
}

func (err UndefinedVarsError) Error() string {
	return fmt.Sprintf("undefined vars: %s", strings.Join(err.Vars, ", "))
}

type UnusedVarsError struct {
	Vars []string
}

func (err UnusedVarsError) Error() string {
	return fmt.Sprintf("unused vars: %s", strings.Join(err.Vars, ", "))
}
