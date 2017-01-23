package dbng

import "github.com/concourse/atc"

type Resource struct {
	ID     int
	Name   string
	Type   string
	Source atc.Source
}
