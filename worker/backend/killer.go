package backend

import (
	"context"
	"syscall"
	"time"

	"github.com/containerd/containerd"
)

type Killer interface {

	// Kill delivers a signal to the task, waiting for a maximum period for
	// it to respond back with an exit status.
	//
	Kill(
		ctx context.Context,
		task containerd.Task,
	) (err error)
}

// OneShot (TODO rename)
//
type OneShot struct {
	Signal  syscall.Signal
	Timeout time.Duration
}

func (k OneShot) Kill(
	ctx context.Context,
	task containerd.Task,
) error {
	return nil
}

// GracefulKiller
//
type GracefulKiller struct {
	GracefulPeriod, UngracefulPeriod time.Duration
	GracefulSignal, UngracefulSignal syscall.Signal
}

func (k GracefulKiller) Kill(
	ctx context.Context,
	task containerd.Task,
) error {
	return nil
}
