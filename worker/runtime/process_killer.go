package runtime

import (
	"context"
	"fmt"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd"
)

//counterfeiter:generate . ProcessKiller

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

	procWaitStatus, err := proc.Wait(waitCtx)
	if err != nil {
		return fmt.Errorf("proc wait: %w", err)
	}

	err = proc.Kill(ctx, signal)
	if err != nil {
		return fmt.Errorf("proc kill w/ signal %d: %w", signal, err)
	}

	// Note: because the wait context is the same for both channels if the timeout expires
	// the choice of which case to run is nondeterministic
	select {
	case <-waitCtx.Done():
		err = waitCtx.Err()
		if err == context.DeadlineExceeded {
			return ErrGracePeriodTimeout
		}

		return fmt.Errorf("waitctx done: %w", err)
	case exitStatus := <-procWaitStatus:
		err = exitStatus.Error()
		if err != nil {
			if strings.Contains(err.Error(), "context deadline exceeded") {
				return ErrGracePeriodTimeout
			}
			return fmt.Errorf("waiting for exit status from grpc: %w", err)
		}
	}

	return nil
}
