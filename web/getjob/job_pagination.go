package getjob

import (
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web/pagination"
)

type Paginator struct {
	PaginatorDB JobPaginatorDB
}

//go:generate counterfeiter . JobPaginatorDB
type JobPaginatorDB interface {
	GetJobBuildsCursor(jobName string, startingID int, resultsGreaterThanStartingID bool, limit int) ([]db.Build, bool, error)
	GetJobBuildsMaxID(jobName string) (int, error)
}

func (p Paginator) PaginateJobBuilds(jobName string, startingJobBuildID int, resultsGreaterThanStartingID bool) ([]db.Build, pagination.PaginationData, error) {
	var paginationData pagination.PaginationData

	maxID, _ := p.PaginatorDB.GetJobBuildsMaxID(jobName)

	if startingJobBuildID == 0 && !resultsGreaterThanStartingID {
		startingJobBuildID = maxID
	}

	builds, moreResultsInGivenDirection, err := p.PaginatorDB.GetJobBuildsCursor(jobName, startingJobBuildID, resultsGreaterThanStartingID, 100)
	if err != nil {
		return []db.Build{}, pagination.PaginationData{}, err
	}

	if len(builds) > 0 {
		paginationData = pagination.NewPaginationData(
			resultsGreaterThanStartingID,
			moreResultsInGivenDirection,
			maxID,
			builds[0].ID,
			builds[len(builds)-1].ID,
		)
	} else {
		paginationData = pagination.PaginationData{}
	}

	return builds, paginationData, nil
}
