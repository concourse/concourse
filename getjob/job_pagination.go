package getjob

import (
	"github.com/concourse/atc/db"
)

type Paginator struct {
	PaginatorDB JobPaginatorDB
}

//go:generate counterfeiter . JobPaginatorDB
type JobPaginatorDB interface {
	GetJobBuildsCursor(jobName string, startingID int, resultsGreaterThanStartingID bool, limit int) ([]db.Build, bool, error)
	GetJobBuildsMaxID(jobName string) (int, error)
}

func (p Paginator) PaginateJobBuilds(jobName string, startingJobBuildID int, resultsGreaterThanStartingID bool) ([]db.Build, PaginationData, error) {
	var paginationData PaginationData

	maxID, _ := p.PaginatorDB.GetJobBuildsMaxID(jobName)

	if startingJobBuildID == 0 && !resultsGreaterThanStartingID {
		startingJobBuildID = maxID
	}

	builds, moreResultsInGivenDirection, err := p.PaginatorDB.GetJobBuildsCursor(jobName, startingJobBuildID, resultsGreaterThanStartingID, 100)
	if err != nil {
		return []db.Build{}, PaginationData{}, err
	}

	if len(builds) > 0 {
		paginationData = NewPaginationData(
			resultsGreaterThanStartingID,
			moreResultsInGivenDirection,
			maxID,
			builds[0].ID,
			builds[len(builds)-1].ID,
		)
	} else {
		paginationData = PaginationData{}
	}

	return builds, paginationData, nil
}

func NewPaginationData(
	resultsGreaterThanStartingID bool,
	moreResultsInGivenDirection bool,
	dbMaxID int,
	maxIDFromResults int,
	minIDFromResults int,
) PaginationData {
	return PaginationData{
		resultsGreaterThanStartingID: resultsGreaterThanStartingID,
		moreResultsInGivenDirection:  moreResultsInGivenDirection,
		dbMaxID:                      dbMaxID,
		maxIDFromResults:             maxIDFromResults,
		minIDFromResults:             minIDFromResults,
	}
}

type PaginationData struct {
	resultsGreaterThanStartingID bool
	moreResultsInGivenDirection  bool
	dbMaxID                      int
	maxIDFromResults             int
	minIDFromResults             int
}

func (pd PaginationData) HasOlder() bool {
	return pd.resultsGreaterThanStartingID || pd.moreResultsInGivenDirection
}

func (pd PaginationData) HasNewer() bool {
	return pd.dbMaxID > pd.maxIDFromResults
}

func (pd PaginationData) HasPagination() bool {
	return pd.HasNewer() || pd.HasOlder()
}

func (pd PaginationData) NewerStartID() int {
	return pd.maxIDFromResults + 1
}

func (pd PaginationData) OlderStartID() int {
	return pd.minIDFromResults - 1
}
