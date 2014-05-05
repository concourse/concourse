package jobs

import "github.com/winston-ci/winston/resources"

type Job struct {
	Name string

	BuildConfigPath string

	Inputs []resources.Resource
}
