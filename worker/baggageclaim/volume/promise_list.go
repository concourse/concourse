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
	promises map[string]Promise

	sync.Mutex
}

func NewPromiseList() PromiseList {
	return &promiseList{
		promises: make(map[string]Promise),
	}
}

func (l *promiseList) AddPromise(handle string, promise Promise) error {
	l.Lock()
	defer l.Unlock()

	if _, exists := l.promises[handle]; exists {
		return ErrPromiseAlreadyExists
	}

	l.promises[handle] = promise

	return nil
}

func (l *promiseList) GetPromise(handle string) Promise {
	l.Lock()
	defer l.Unlock()

	return l.promises[handle]
}

func (l *promiseList) RemovePromise(handle string) {
	l.Lock()
	defer l.Unlock()

	delete(l.promises, handle)
}
