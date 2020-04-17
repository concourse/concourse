package db

import "fmt"

//go:generate counterfeiter . Conflict

type Conflict interface {
	error
	Conflict() string
}

type conflict string

func (c conflict) Error() string {
	return fmt.Sprintf("conflict error: %s", string(c))
}

func (c conflict) Conflict() string {
	return string(c)
}
