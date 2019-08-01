package gclient

import (
	"context"
	"io"
	"strings"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/atc/worker/gclient/connection"
)

type RetryableConnection struct {
	connection.Connection
}

func NewRetryableConnection(connection connection.Connection) connection.Connection {
	return &RetryableConnection{
		Connection: connection,
	}
}

func (conn *RetryableConnection) Run(ctx context.Context, handle string, processSpec garden.ProcessSpec, processIO garden.ProcessIO) (garden.Process, error) {
	innerProcess, err := conn.Connection.Run(ctx, handle, processSpec, processIO)
	if err != nil {
		return nil, err
	}

	return &retryableProcess{
		Process: innerProcess,

		rehydrate: func() (garden.Process, error) {
			return conn.Connection.Attach(ctx, handle, innerProcess.ID(), processIO)
		},
	}, nil
}

func (conn *RetryableConnection) Attach(ctx context.Context, handle string, processID string, processIO garden.ProcessIO) (garden.Process, error) {
	innerProcess, err := conn.Connection.Attach(ctx, handle, processID, processIO)
	if err != nil {
		return nil, err
	}

	return &retryableProcess{
		Process: innerProcess,

		rehydrate: func() (garden.Process, error) {
			return conn.Connection.Attach(ctx, handle, processID, processIO)
		},
	}, nil
}

type retryableProcess struct {
	garden.Process

	rehydrate func() (garden.Process, error)
}

func (process *retryableProcess) Wait() (int, error) {
	for {
		status, err := process.Process.Wait()
		if err == nil {
			return status, nil
		}

		if !strings.HasSuffix(err.Error(), io.EOF.Error()) {
			return 0, err
		}

		process.Process, err = process.rehydrate()
		if err != nil {
			return 0, err
		}
	}
}

func (process *retryableProcess) Signal(sig garden.Signal) error {
	for {
		err := process.Process.Signal(sig)
		if err == nil {
			return nil
		}

		if strings.Contains(err.Error(), "use of closed network connection") {
			return err
		}

		process.Process, err = process.rehydrate()
		if err != nil {
			return err
		}
	}
}

func (process *retryableProcess) SetTTY(tty garden.TTYSpec) error {
	for {
		err := process.Process.SetTTY(tty)
		if err == nil {
			return nil
		}

		if strings.Contains(err.Error(), "use of closed network connection") {
			return err
		}

		process.Process, err = process.rehydrate()
		if err != nil {
			return err
		}
	}
}
