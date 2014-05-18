package builds

type BuildStatus int

const (
	BuildStatusInvalid BuildStatus = iota
	BuildStatusPending
	BuildStatusRunning
	BuildStatusSucceeded
	BuildStatusFailed
	BuildStatusErrored
)

type Build struct {
	ID int

	Status BuildStatus
}
