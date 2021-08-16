package volume

import (
	"errors"
	"sync"
)

var ErrPromiseCanceled = errors.New("promise was canceled")
var ErrPromiseNotPending = errors.New("promise is not pending")
var ErrPromiseStillPending = errors.New("promise is still pending")

type Promise interface {
	IsPending() bool
	GetValue() (Volume, error, error)
	Fulfill(Volume) error
	Reject(error) error
}

type promise struct {
	volume *Volume
	err    error
	cancel chan struct{}

	sync.RWMutex
}

func NewPromise() Promise {
	return &promise{
		volume: nil,
		err:    nil,
		cancel: make(chan struct{}),
	}
}

func (p *promise) IsPending() bool {
	p.RLock()
	defer p.RUnlock()

	return p.isPending()
}

func (p *promise) isPending() bool {
	return p.volume == nil && p.err == nil
}

func (p *promise) GetValue() (Volume, error, error) {
	p.RLock()
	defer p.RUnlock()

	if p.IsPending() {
		return Volume{}, nil, ErrPromiseStillPending
	}

	if p.volume == nil {
		return Volume{}, p.err, nil
	}
	return *p.volume, p.err, nil
}

func (p *promise) Fulfill(volume Volume) error {
	p.Lock()
	defer p.Unlock()

	if !p.isPending() {
		if p.err == ErrPromiseCanceled {
			return ErrPromiseCanceled
		}
		return ErrPromiseNotPending
	}

	p.volume = &volume

	return nil
}

func (p *promise) Reject(err error) error {
	p.Lock()
	defer p.Unlock()

	if !p.isPending() {
		return ErrPromiseNotPending
	}

	p.err = err

	return nil
}
