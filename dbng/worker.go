package dbng

import "errors"

type WorkerState string

const (
	WorkerStateRunning = WorkerState("running")
	WorkerStateStalled = WorkerState("stalled")
	WorkerStateLanding = WorkerState("landing")
)

var (
	ErrWorkerNotPresent = errors.New("worker-not-present-in-db")
)

type Worker struct {
	Name       string
	GardenAddr *string
	State      WorkerState
}
