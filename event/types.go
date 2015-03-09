package event

import "github.com/concourse/atc"

const (
	EventTypeInvalid atc.EventType = ""

	// build log (e.g. from input or build execution)
	EventTypeLog atc.EventType = "log"

	// build status change (e.g. 'started', 'succeeded')
	EventTypeStatus atc.EventType = "status"

	// build initializing (all inputs fetched; fetching image)
	EventTypeInitializeExecute atc.EventType = "initialize-execute"

	// build execution started
	EventTypeStartExecute atc.EventType = "start-execute"

	// build execution finished
	EventTypeFinishExecute atc.EventType = "finish-execute"

	// finished getting something
	EventTypeFinishGet atc.EventType = "finish-get"

	// finished putting something
	EventTypeFinishPut atc.EventType = "finish-put"

	// error occurred
	EventTypeError atc.EventType = "error"
)
