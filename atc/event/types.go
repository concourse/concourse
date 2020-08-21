package event

import "github.com/concourse/concourse/atc"

const (
	// build log (e.g. from input or build execution)
	EventTypeLog atc.EventType = "log"

	// build status change (e.g. 'started', 'succeeded')
	EventTypeStatus atc.EventType = "status"

	// a step (get/put/task) selected worker
	EventTypeSelectedWorker atc.EventType = "selected-worker"

	// task execution started
	EventTypeStartTask atc.EventType = "start-task"

	// task initializing (all inputs fetched; fetching image)
	EventTypeInitializeTask atc.EventType = "initialize-task"

	// task execution finished
	EventTypeFinishTask atc.EventType = "finish-task"

	// initialize getting something
	EventTypeInitializeGet atc.EventType = "initialize-get"

	// started getting something
	EventTypeStartGet atc.EventType = "start-get"

	// finished getting something
	EventTypeFinishGet atc.EventType = "finish-get"

	// initialize putting something
	EventTypeInitializePut atc.EventType = "initialize-put"

	// started putting something
	EventTypeStartPut atc.EventType = "start-put"

	// finished putting something
	EventTypeFinishPut atc.EventType = "finish-put"

	EventTypeSetPipelineChanged atc.EventType = "set-pipeline-changed"

	// initialize step
	EventTypeInitialize atc.EventType = "initialize"

	// started step
	EventTypeStart atc.EventType = "start"

	// finished step
	EventTypeFinish atc.EventType = "finish"

	// error occurred
	EventTypeError atc.EventType = "error"
)
