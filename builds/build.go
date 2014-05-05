package builds

type BuildState int

const (
	BuildStateInvalid BuildState = iota
	BuildStatePending
	BuildStateRunning
	BuildStateSucceeded
	BuildStateFailed
	BuildStateErrored
)

type Build struct {
	ID int

	State BuildState
}
