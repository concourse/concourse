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
