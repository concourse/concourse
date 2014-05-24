package resources

import "github.com/winston-ci/winston/config"

type Checker interface {
	CheckResource(config.Resource) []config.Resource
}
