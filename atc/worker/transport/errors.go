package transport

import "fmt"

type WorkerMissingError struct {
	WorkerName string
}

func (e WorkerMissingError) Error() string {
	return fmt.Sprintf("worker %s disappeared while trying to reach it", e.WorkerName)
}

type WorkerUnreachableError struct {
	WorkerName  string
	WorkerState string
}

func (e WorkerUnreachableError) Error() string {
	return fmt.Sprintf("worker '%s' is unreachable (state is '%s')", e.WorkerName, e.WorkerState)
}
