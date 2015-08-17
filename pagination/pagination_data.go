package pagination

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
