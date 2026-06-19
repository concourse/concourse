package gardenruntime

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagerctx"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker/gardenruntime/gclient"
)

// How long Process.Wait waits for a process to exit after sending it a SIGTERM
// (on abort) before forcibly tearing things down. This bounds aborts even when
// the worker has disconnected: rather than blocking forever waiting for a
// process stream that a dead worker will never complete, we force-kill and
// cancel the stream once this elapses.
var gardenProcessStopGracePeriod = 10 * time.Second

type Process struct {
	GardenContainer gclient.Container
	GardenProcess   garden.Process

	// cancelStream cancels the context that backs the process output stream.
	// Cancelling it closes the underlying (possibly hung) connection, which
	// unblocks GardenProcess.Wait. It may be nil for processes that were not
	// created via Container.Run/Attach (e.g. ExitedProcess).
	cancelStream context.CancelFunc
}

func (p Process) ID() string {
	return p.GardenProcess.ID()
}

func (p Process) Wait(ctx context.Context) (runtime.ProcessResult, error) {
	logger := lagerctx.FromContext(ctx).Session("process-wait")

	if p.cancelStream != nil {
		defer p.cancelStream()
	}

	type result struct {
		exitStatus int
		err        error
	}
	waitResult := make(chan result, 1)

	go func() {
		exitStatus, err := p.GardenProcess.Wait()
		waitResult <- result{exitStatus: exitStatus, err: err}
	}()

	select {
	case <-ctx.Done():
		go p.gracefulStop(logger)

		select {
		case <-waitResult:
			// Exited within the grace period.
		case <-time.After(gardenProcessStopGracePeriod):
			logger.Info("grace-period-expired-forcing-stop")
			go p.forcefulStop(logger)
			if p.cancelStream != nil {
				p.cancelStream()
			}

			select {
			case <-waitResult:
			case <-time.After(gardenProcessStopGracePeriod):
				logger.Info("forced-stop-timed-out-abandoning-wait")
			}
		}
		return runtime.ProcessResult{}, ctx.Err()
	case r := <-waitResult:
		if r.err != nil {
			return runtime.ProcessResult{}, fmt.Errorf("wait for process completion: %w", r.err)
		}
		p.GardenContainer.SetProperty(exitStatusPropertyName, strconv.Itoa(r.exitStatus))
		return runtime.ProcessResult{ExitStatus: r.exitStatus}, nil
	}
}

func (p Process) gracefulStop(logger lager.Logger) {
	if err := p.GardenContainer.Stop(false); err != nil {
		logger.Error("failed-to-stop-container", err)
	}
}

func (p Process) forcefulStop(logger lager.Logger) {
	if err := p.GardenContainer.Stop(true); err != nil {
		logger.Error("failed-to-force-stop-container", err)
	}
}

func (p Process) SetTTY(tty runtime.TTYSpec) error {
	return p.GardenProcess.SetTTY(toGardenTTYSpec(tty))
}

type ExitedProcess struct {
	id     string
	Result runtime.ProcessResult
}

func (p ExitedProcess) ID() string {
	return p.id
}

func (p ExitedProcess) Wait(ctx context.Context) (runtime.ProcessResult, error) {
	return p.Result, nil
}

func (p ExitedProcess) SetTTY(tty runtime.TTYSpec) error {
	return nil
}
