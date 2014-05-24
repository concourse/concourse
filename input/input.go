package input

import "github.com/winston-ci/winston/config"

type ResourceChecker interface {
	Check(config.Resource) []config.Resource
}
