package volume

import (
	"errors"
	"sync"
)

var ErrPromiseAlreadyExists = errors.New("promise already exists in list")

type PromiseList interface {
	// AddPromise adds a new promise with the given handle.
	// Returns ErrPromiseAlreadyExists if a promise with the handle already exists.
	AddPromise(handle string, promise Promise) error

	// GetPromise returns the promise with the given handle.
	// Returns nil if no promise exists with the handle.
	GetPromise(handle string) Promise

	// RemovePromise removes the promise with the given handle.
	// If no promise exists with the handle, this is a no-op.
	RemovePromise(handle string)
}

type promiseList struct {
	promises sync.Map
}

func NewPromiseList() PromiseList {
	return &promiseList{}
}

// AddPromise adds a new promise with the given handle.
// Returns ErrPromiseAlreadyExists if a promise with the handle already exists.
func (l *promiseList) AddPromise(handle string, promise Promise) error {
	// LoadOrStore returns the existing value if the key exists
	// and a boolean indicating if the value was loaded or stored
	if existingPromise, loaded := l.promises.LoadOrStore(handle, promise); loaded {
		// Promise already exists with this handle
		_ = existingPromise // avoid compiler warning about unused variable
		return ErrPromiseAlreadyExists
	}

	return nil
}

// GetPromise returns the promise with the given handle.
// Returns nil if no promise exists with the handle.
func (l *promiseList) GetPromise(handle string) Promise {
	// Load returns the value stored in the map for a key, or nil if no value is present
	if promise, exists := l.promises.Load(handle); exists {
		return promise.(Promise)
	}

	// Return nil explicitly when the promise doesn't exist
	return nil
}

// RemovePromise removes the promise with the given handle.
// If no promise exists with the handle, this is a no-op.
func (l *promiseList) RemovePromise(handle string) {
	l.promises.Delete(handle)
}
