package runtime

import (
	"context"
	"errors"
	"fmt"

	"code.cloudfoundry.org/garden"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
)

type Process struct {
	process     containerd.Process
	exitStatusC <-chan containerd.ExitStatus
	stdin       *stdinWrapper
}

func NewProcess(
	p containerd.Process,
	ch <-chan containerd.ExitStatus,
	in *stdinWrapper,
) *Process {
	return &Process{
		process:     p,
		exitStatusC: ch,
		stdin:       in,
	}
}

var _ garden.Process = (*Process)(nil)

// Id retrieves the ID associated with this process.
//
func (p *Process) ID() string {
	return p.process.ID()
}

// Wait for the process to terminate (either naturally, or from a signal), and
// once done, delete it.
//
func (p *Process) Wait() (int, error) {
	status := <-p.exitStatusC
	err := status.Error()
	if err != nil {
		return 0, fmt.Errorf("waiting for exit status: %w", err)
	}

	p.process.IO().Wait()

	_, err = p.process.Delete(context.Background())
	// ignore "not found" errors - the process was already deleted
	if err != nil && !errors.Is(err, errdefs.ErrNotFound) {
		return 0, fmt.Errorf("delete process: %w", err)
	}

	if p.stdin != nil {
		p.stdin.Close()
	}

	return int(status.ExitCode()), nil
}

// SetTTY resizes the process' terminal dimensions.
//
func (p *Process) SetTTY(spec garden.TTYSpec) error {
	if spec.WindowSize == nil {
		return nil
	}

	err := p.process.Resize(context.Background(),
		uint32(spec.WindowSize.Columns),
		uint32(spec.WindowSize.Rows),
	)
	if err != nil {
		return fmt.Errorf("resize: %w", err)
	}

	return nil
}

// Signal - Not Implemented
//
func (p *Process) Signal(signal garden.Signal) (err error) {
	err = ErrNotImplemented
	return
}
