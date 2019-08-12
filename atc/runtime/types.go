package runtime

const (
	InitializingEvent = "Initializing"
	StartingEvent     = "Starting"
	FinishedEvent     = "Finished"
)

type Event struct {
	EventType  string
	ExitStatus int
}
