package dbng

type BuildStatus string

const (
	BuildStatusPending   BuildStatus = "pending"
	BuildStatusStarted   BuildStatus = "started"
	BuildStatusAborted   BuildStatus = "aborted"
	BuildStatusSucceeded BuildStatus = "succeeded"
	BuildStatusFailed    BuildStatus = "failed"
	BuildStatusErrored   BuildStatus = "errored"
)
