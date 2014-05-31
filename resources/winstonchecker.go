package resources

import (
	"reflect"

	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/config"
	"github.com/winston-ci/winston/db"
)

type WinstonChecker struct {
	db   db.DB
	jobs []string
}

func NewWinstonChecker(db db.DB, jobs []string) Checker {
	return &WinstonChecker{db, jobs}
}

func (checker *WinstonChecker) CheckResource(resource config.Resource, from builds.Version) []builds.Version {
	commonOutputs, err := checker.db.GetCommonOutputs(checker.jobs, resource.Name)
	if err != nil {
		return nil
	}

	startFrom := 0
	for i, source := range commonOutputs {
		if reflect.DeepEqual(source, from) {
			startFrom = i + 1
		}
	}

	return commonOutputs[startFrom:]
}
