package backend

import (
	"context"
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

	// GracefulPeriod is the duration by which a graceful killer would let a
	// set of processes finish by themselves before going ungraceful.
	//
	GracefulPeriod = 10 * time.Second
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Killer

// Killer terminates tasks.
//
type Killer interface {
	// Kill terminates a task.
	//
	Kill(ctx context.Context, task containerd.Task) (err error)
}

// UngracefulKiller terminates a task's processes without giving them chance to
// gracefully finish themselves first.
//
type UngracefulKiller struct{}

func NewUngracefulKiller() UngracefulKiller {
	return UngracefulKiller{}
}

// Kill delivers a signal to the init process, letting it die, bringing its
// sibling processes together with it through an implict SIGKILL delivered by
// the kernel during the termination of its PID namespace.
//
// 	container............
// 	.
// 	.	init proc	<- takes an explicit SIGKILL
// 	.	/opt/resource/in
// 	.         git clone
// 	.
//
//		- once `init` is gone (guaranteed via SIGKILL), all other
//		processes in the pid namespace get a SIGKILL too.
//
//
// ref: http://man7.org/linux/man-pages/man7/pid_namespaces.7.html
//
func (k UngracefulKiller) Kill(
	ctx context.Context,
	task containerd.Task,
) error {
	return killTaskInitProc(ctx, task, UngracefulSignal)
}

// GracefulKiller terminates the processes in a task by first letting them
// terminate themselves by their own means, and if and only if they don't do it
// in time, ungracefully force them to be shutdown (via SIGKILL).
//
// ps.: only processes created through `task.Exec` are targetted to receive the
//      the first graceful signal.
//
// pps.: ungraceful finish is driven by terminating the init process in the pid
//       namespace. (see `UngracefulKiller`).
//
//
type GracefulKiller struct {
	GracefulPeriod time.Duration
}

func NewGracefulKiller() GracefulKiller {
	return GracefulKiller{
		GracefulPeriod: GracefulPeriod,
	}
}

// Kill delivers a graceful signal to each exec'ed process in the task, waits
// for them to finish on time, and, if not, ungracefully kills them.
//
func (k GracefulKiller) Kill(ctx context.Context, task containerd.Task) error {
	err := killTaskExecedProcesses(ctx, task, GracefulSignal)
	if err != nil {
		if err != ErrGracePeriodTimeout {
			return fmt.Errorf("kill task execed processes: %w", err)
		}
	}

	err = killTaskInitProc(ctx, task, UngracefulSignal)
	if err != nil {
		return fmt.Errorf("kill task init proc: %w", err)
	}

	return nil
}

// killTaskInitProc terminates the process that corresponds to the root of the
// sandbox (the init proc).
//
func killTaskInitProc(ctx context.Context, task containerd.Task, signal syscall.Signal) error {
	err := killProcess(ctx, task, signal)
	if err != nil {
		return fmt.Errorf("kill process: %w", err)
	}

	return nil
}

// killTaskProcesses delivers a signal to every live process that has been
// created through a `task.Exec`.
//
func killTaskExecedProcesses(ctx context.Context, task containerd.Task, signal syscall.Signal) error {
	procs, err := taskExecedProcesses(ctx, task)
	if err != nil {
		return fmt.Errorf("task execed processes: %w", err)
	}

	err = killProcesses(ctx, procs, signal)
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
func killProcesses(ctx context.Context, procs []containerd.Process, signal syscall.Signal) error {

	// TODO - this could (probably *should*) be concurrent
	//

	for _, proc := range procs {
		err := killProcess(ctx, proc, signal)
		if err != nil {
			return err
		}
	}

	return nil
}

// killProcess takes care of delivering a termination signal to a process and
// waiting for its exit status.
//
func killProcess(ctx context.Context, proc containerd.Process, signal syscall.Signal) error {
	// TODO inject that period
	//
	waitCtx, cancel := context.WithTimeout(ctx, GracefulPeriod)
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
	case <-ctx.Done():
		return fmt.Errorf("ctx done: %w", ctx.Err())
	case <-statusC:
		// TODO handle possible status error
	}

	return nil
}
