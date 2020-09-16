package component

import "context"

// Drainable is an optional interface which component runners can implement in
// order to perform something on shutdown.
type Drainable interface {
	// Drain is invoked on shutdown.
	Drain(context.Context)
}
