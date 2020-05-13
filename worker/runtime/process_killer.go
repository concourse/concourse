package runtime

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"github.com/containerd/containerd"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . ProcessKiller

type ProcessKiller interface {

	// Kill terminates a single process.
	//
	Kill(
		ctx context.Context,
		proc containerd.Process,
		signal syscall.Signal,
		waitPeriod time.Duration,
	) error
}

type processKiller struct{}

func NewProcessKiller() *processKiller {
	return &processKiller{}
}

// Kill delivers a signal to a process, waiting for a maximum of `waitPeriod`
// for a status to be reported back.
//
// In case no status is reported within the grace period time span,
// ErrGracePeriodTimeout is returned.
//
// ps.: even in the case of a SIGKILL being used as the signal, `Kill` will
//      wait for a `waitPeriod` for a status to be reported back. This way one
//      can detect cases where not even a SIGKILL changes the status of a
//      process (e.g., if the process is frozen due to a blocking syscall that
//      never returns, or because it's suspended - see [1]).
//
// [1]: https://github.com/concourse/concourse/issues/4477
//
func (p processKiller) Kill(
	ctx context.Context,
	proc containerd.Process,
	signal syscall.Signal,
	waitPeriod time.Duration,
) error {
	waitCtx, cancel := context.WithTimeout(ctx, waitPeriod)
	defer cancel()

	statusC, err := proc.Wait(waitCtx)
	if err != nil {
		return fmt.Errorf("proc wait: %w", err)
	}

	err = proc.Kill(ctx, signal)
	if err != nil {
		return fmt.Errorf("proc kill w/ signal %d: %w", signal, err)
	}

	select {
	case <-waitCtx.Done():
		err = waitCtx.Err()
		if err == context.DeadlineExceeded {
			return ErrGracePeriodTimeout
		}

		return fmt.Errorf("waitctx done: %w", err)
	case status := <-statusC:
		err = status.Error()
		if err != nil {
			return fmt.Errorf("waiting for exit status: %w", err)
		}
	}

	return nil
}
