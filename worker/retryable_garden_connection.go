package worker

import (
	"io"
	"strings"

	"time"

	"code.cloudfoundry.org/garden"
	gconn "code.cloudfoundry.org/garden/client/connection"
)

//go:generate counterfeiter . Sleeper

type Sleeper interface {
	Sleep(time.Duration)
}

type RetryableConnection struct {
	gconn.Connection
}

func NewRetryableConnection(connection gconn.Connection) *RetryableConnection {
	return &RetryableConnection{
		Connection: connection,
	}
}

func (conn *RetryableConnection) Run(handle string, processSpec garden.ProcessSpec, processIO garden.ProcessIO) (garden.Process, error) {
	innerProcess, err := conn.Connection.Run(handle, processSpec, processIO)
	if err != nil {
		return nil, err
	}

	return &retryableProcess{
		Process: innerProcess,

		rehydrate: func() (garden.Process, error) {
			return conn.Attach(handle, innerProcess.ID(), processIO)
		},
	}, nil
}

func (conn *RetryableConnection) Attach(handle string, processID string, processIO garden.ProcessIO) (garden.Process, error) {
	innerProcess, err := conn.Connection.Attach(handle, processID, processIO)
	if err != nil {
		return nil, err
	}

	return &retryableProcess{
		Process: innerProcess,

		rehydrate: func() (garden.Process, error) {
			return conn.Attach(handle, processID, processIO)
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
