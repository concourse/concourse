package runtime

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/runtime/v2/runc/options"
	"github.com/containerd/typeurl"
)

const (
	// GracefulSignal is the signal sent to processes when giving them the
	// opportunity to shut themselves down by their own means.
	//
	GracefulSignal = syscall.SIGTERM

	// UngracefulSignal is the signal sent to the init process in the pid
	// namespace to force its shutdown.
	//
	UngracefulSignal = syscall.SIGKILL

	// GracePeriod is the duration by which a graceful killer would let a
	// set of processes finish by themselves before going ungraceful.
	//
	GracePeriod = 10 * time.Second
)

type KillBehaviour bool

const (
	KillGracefully   KillBehaviour = false
	KillUngracefully KillBehaviour = true
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Killer

// Killer terminates tasks.
//
type Killer interface {
	// Kill terminates a task either with a specific behaviour.
	//
	Kill(
		ctx context.Context,
		task containerd.Task,
		behaviour KillBehaviour,
	) error
}

// killer terminates the processes exec'ed in a task.
//
// Only processes created through `task.Exec` are targetted to receive the the
// first signals it delivers.
//
type killer struct {
	gracePeriod   time.Duration
	processKiller ProcessKiller
}

// KillerOpt is a functional option that modifies the behavior of a killer.
//
type KillerOpt func(k *killer)

// WithProcessKiller modifies the default process killer used by the task
// killer.
//
func WithProcessKiller(f ProcessKiller) KillerOpt {
	return func(k *killer) {
		k.processKiller = f
	}
}

// WithGracePeriod configures the grace period used when waiting for a process
// to be gracefully finished.
//
func WithGracePeriod(p time.Duration) KillerOpt {
	return func(k *killer) {
		k.gracePeriod = p
	}
}

func NewKiller(opts ...KillerOpt) *killer {
	k := &killer{
		gracePeriod:   GracePeriod,
		processKiller: NewProcessKiller(),
	}

	for _, opt := range opts {
		opt(k)
	}

	return k
}

// Kill delivers a signal to each exec'ed process in the task.
//
func (k killer) Kill(ctx context.Context, task containerd.Task, behaviour KillBehaviour) error {
	switch behaviour {
	case KillGracefully:
		success, err := k.gracefullyKill(ctx, task)
		if err != nil {
			return fmt.Errorf("graceful kill: %w", err)
		}
		if !success {
			err := k.ungracefullyKill(ctx, task)
			if err != nil {
				return fmt.Errorf("ungraceful kill: %w", err)
			}
		}
	case KillUngracefully:
		err := k.ungracefullyKill(ctx, task)
		if err != nil {
			return fmt.Errorf("ungraceful kill: %w", err)
		}
	}
	return nil
}

func (k killer) ungracefullyKill(ctx context.Context, task containerd.Task) error {
	err := k.killTaskExecedProcesses(ctx, task, UngracefulSignal)
	if err != nil {
		return fmt.Errorf("ungraceful kill task execed processes: %w", err)
	}

	return nil
}

func (k killer) gracefullyKill(ctx context.Context, task containerd.Task) (bool, error) {
	err := k.killTaskExecedProcesses(ctx, task, GracefulSignal)
	switch {
	case errors.Is(err, ErrGracePeriodTimeout):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("kill task execed processes: %w", err)
	}

	return true, nil
}

// killTaskProcesses delivers a signal to every live process that has been
// created through a `task.Exec`.
//
func (k killer) killTaskExecedProcesses(ctx context.Context, task containerd.Task, signal syscall.Signal) error {
	procs, err := taskExecedProcesses(ctx, task)
	if err != nil {
		return fmt.Errorf("task execed processes: %w", err)
	}

	err = k.killProcesses(ctx, procs, signal)
	if err != nil {
		return fmt.Errorf("kill procs: %w", err)
	}

	return nil
}

// taskProcesses retrieves a task's processes.
//
func taskExecedProcesses(ctx context.Context, task containerd.Task) ([]containerd.Process, error) {
	pids, err := task.Pids(context.Background())
	if err != nil {
		return nil, fmt.Errorf("pid listing: %w", err)
	}

	procs := []containerd.Process{}
	for _, pid := range pids {
		if pid.Info == nil { // init
			continue
		}

		// the protobuf message has a "catch-all" field for `pid.Info`,
		// thus, we need to unmarshal the message ourselves.
		//
		info, err := typeurl.UnmarshalAny(pid.Info)
		if err != nil {
			return nil, fmt.Errorf("proc details unmarshal: %w", err)
		}

		pinfo, ok := info.(*options.ProcessDetails)
		if !ok {
			return nil, fmt.Errorf("unknown proc detail type")
		}

		proc, err := task.LoadProcess(ctx, pinfo.ExecID, cio.Load)
		if err != nil {
			return nil, fmt.Errorf("load process: %w", err)
		}

		procs = append(procs, proc)
	}

	return procs, nil
}

// killProcesses takes care of delivering a termination signal to a set of
// processes and waiting for their statuses.
//
func (k killer) killProcesses(ctx context.Context, procs []containerd.Process, signal syscall.Signal) error {

	// TODO - this could (probably *should*) be concurrent
	//

	for _, proc := range procs {
		err := k.processKiller.Kill(ctx, proc, signal, k.gracePeriod)
		if err != nil {
			return fmt.Errorf("proc kill: %w", err)
		}
	}

	return nil
}
