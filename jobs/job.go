package jobs

import "github.com/winston-ci/prole/api/builds"

type Job struct {
	Name string

	Privileged bool

	BuildConfigPath string

	Inputs []builds.Input
}
