package resources

import (
	"reflect"

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

func (checker *WinstonChecker) CheckResource(resource config.Resource) []config.Resource {
	commonOutputs, err := checker.db.GetCommonOutputs(checker.jobs, resource.Name)
	if err != nil {
		return nil
	}

	startFrom := 0
	resources := make([]config.Resource, len(commonOutputs))
	for i, source := range commonOutputs {
		if reflect.DeepEqual(source, resource.Source) {
			startFrom = i + 1
		}

		resources[i] = resource
		resources[i].Source = source
	}

	return resources[startFrom:]
}
