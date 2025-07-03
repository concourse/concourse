package volume

import (
	"errors"
	"sync"
)

var ErrPromiseAlreadyExists = errors.New("promise already exists in list")

type PromiseList interface {
	AddPromise(handle string, promise Promise) error

	GetPromise(handle string) Promise

	RemovePromise(handle string)
}

type promiseList struct {
	promises sync.Map
}

func NewPromiseList() PromiseList {
	return &promiseList{}
}

func (l *promiseList) AddPromise(handle string, promise Promise) error {
	if _, loaded := l.promises.LoadOrStore(handle, promise); loaded {
		return ErrPromiseAlreadyExists
	}

	return nil
}

func (l *promiseList) GetPromise(handle string) Promise {
	if promise, exists := l.promises.Load(handle); exists {
		return promise.(Promise)
	}

	return nil
}

func (l *promiseList) RemovePromise(handle string) {
	l.promises.Delete(handle)
}
