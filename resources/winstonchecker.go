package resources

import (
	"reflect"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
)

type WinstonChecker struct {
	db   db.DB
	jobs []string
}

func NewWinstonChecker(db db.DB, jobs []string) Checker {
	return &WinstonChecker{db, jobs}
}

func (checker *WinstonChecker) CheckResource(resource config.Resource, from builds.Version) ([]builds.Version, error) {
	commonOutputs, err := checker.db.GetCommonOutputs(checker.jobs, resource.Name)
	if err != nil {
		return nil, err
	}

	startFrom := 0
	for i, source := range commonOutputs {
		if reflect.DeepEqual(source, from) {
			startFrom = i + 1
		}
	}

	return commonOutputs[startFrom:], nil
}
