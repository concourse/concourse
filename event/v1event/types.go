package v1event

import "github.com/concourse/atc"

const (
	EventTypeInvalid atc.EventType = ""

	// build log (e.g. from input or build execution)
	EventTypeLog atc.EventType = "log"

	// build status change (e.g. 'started', 'succeeded')
	EventTypeStatus atc.EventType = "status"

	// build initializing (all inputs fetched; fetching image)
	EventTypeInitialize atc.EventType = "initialize"

	// build execution started
	EventTypeStart atc.EventType = "start"

	// build execution finished
	EventTypeFinish atc.EventType = "finish"

	// error occurred
	EventTypeError atc.EventType = "error"

	// input fetched
	EventTypeInput atc.EventType = "input"

	// output completed
	EventTypeOutput atc.EventType = "output"
)
