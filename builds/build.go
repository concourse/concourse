package builds

type Status string

const (
	StatusPending   Status = "pending"
	StatusStarted   Status = "started"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusErrored   Status = "errored"
)

type Build struct {
	ID int

	Status Status
}

type ByID []Build

func (builds ByID) Len() int {
	return len(builds)
}

func (builds ByID) Less(i, j int) bool {
	return builds[i].ID < builds[j].ID
}

func (builds ByID) Swap(i, j int) {
	builds[i], builds[j] = builds[j], builds[i]
}
