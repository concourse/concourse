package worker

import (
	"errors"
	"io"
	"net"
	"strings"
	"syscall"

	"github.com/cloudfoundry-incubator/garden"
	gconn "github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/pivotal-golang/lager"
)

import "time"

//go:generate counterfeiter . Sleeper

type Sleeper interface {
	Sleep(time.Duration)
}

//go:generate counterfeiter . RetryPolicy

type RetryPolicy interface {
	DelayFor(uint) (time.Duration, bool)
}

var retryableErrors = []error{
	syscall.ECONNREFUSED,
	syscall.ECONNRESET,
	syscall.ETIMEDOUT,
	errors.New("i/o timeout"),
	errors.New("no such host"),
	errors.New("remote error: handshake failure"),
}

type RetryableConnection struct {
	gconn.Connection

	Logger            lager.Logger
	Sleeper           Sleeper
	RetryPolicy       RetryPolicy
	ConnectionFactory GardenConnectionFactory
}

func NewRetryableConnection(
	logger lager.Logger,
	sleeper Sleeper,
	retryPolicy RetryPolicy,
	connectionFactory GardenConnectionFactory,
) *RetryableConnection {
	return &RetryableConnection{
		Connection: connectionFactory.BuildConnection(),

		Logger:            logger,
		Sleeper:           sleeper,
		RetryPolicy:       retryPolicy,
		ConnectionFactory: connectionFactory,
	}
}

func (conn *RetryableConnection) Ping() error {
	return conn.Connection.Ping()
}

func (conn *RetryableConnection) Capacity() (garden.Capacity, error) {
	var capacity garden.Capacity

	err := conn.retry(func() error {
		var err error
		capacity, err = conn.Connection.Capacity()
		return err
	})
	if err != nil {
		return garden.Capacity{}, err
	}

	return capacity, nil
}

func (conn *RetryableConnection) List(properties garden.Properties) ([]string, error) {
	var handles []string

	err := conn.retry(func() error {
		var err error
		handles, err = conn.Connection.List(properties)
		return err
	})
	if err != nil {
		return nil, err
	}

	return handles, nil
}

func (conn *RetryableConnection) Info(handle string) (garden.ContainerInfo, error) {
	var info garden.ContainerInfo

	err := conn.retry(func() error {
		var err error
		info, err = conn.Connection.Info(handle)
		return err
	})
	if err != nil {
		return garden.ContainerInfo{}, err
	}

	return info, nil
}

func (conn *RetryableConnection) NetIn(handle string, hostPort, containerPort uint32) (uint32, uint32, error) {
	var resultingHostPort, resultingContainerPort uint32

	err := conn.retry(func() error {
		var err error
		resultingHostPort, resultingContainerPort, err = conn.Connection.NetIn(handle, hostPort, containerPort)
		return err
	})
	if err != nil {
		return 0, 0, err
	}

	return resultingHostPort, resultingContainerPort, nil
}

func (conn *RetryableConnection) NetOut(handle string, rule garden.NetOutRule) error {
	return conn.retry(func() error {
		return conn.Connection.NetOut(handle, rule)
	})
}

func (conn *RetryableConnection) Create(spec garden.ContainerSpec) (string, error) {
	var resultingHandle string

	err := conn.retry(func() error {
		var err error
		resultingHandle, err = conn.Connection.Create(spec)
		return err
	})
	if err != nil {
		return "", err
	}

	return resultingHandle, nil
}

func (conn *RetryableConnection) Destroy(handle string) error {
	return conn.retry(func() error {
		return conn.Connection.Destroy(handle)
	})
}

func (conn *RetryableConnection) Stop(handle string, kill bool) error {
	return conn.retry(func() error {
		return conn.Connection.Stop(handle, kill)
	})
}

func (conn *RetryableConnection) CurrentBandwidthLimits(handle string) (garden.BandwidthLimits, error) {
	var resultingLimits garden.BandwidthLimits

	err := conn.retry(func() error {
		var err error
		resultingLimits, err = conn.Connection.CurrentBandwidthLimits(handle)
		return err
	})
	if err != nil {
		return garden.BandwidthLimits{}, err
	}

	return resultingLimits, nil
}

func (conn *RetryableConnection) CurrentCPULimits(handle string) (garden.CPULimits, error) {
	var resultingLimits garden.CPULimits

	err := conn.retry(func() error {
		var err error
		resultingLimits, err = conn.Connection.CurrentCPULimits(handle)
		return err
	})
	if err != nil {
		return garden.CPULimits{}, err
	}

	return resultingLimits, nil
}

func (conn *RetryableConnection) CurrentDiskLimits(handle string) (garden.DiskLimits, error) {
	var resultingLimits garden.DiskLimits

	err := conn.retry(func() error {
		var err error
		resultingLimits, err = conn.Connection.CurrentDiskLimits(handle)
		return err
	})
	if err != nil {
		return garden.DiskLimits{}, err
	}

	return resultingLimits, nil
}

func (conn *RetryableConnection) CurrentMemoryLimits(handle string) (garden.MemoryLimits, error) {
	var resultingLimits garden.MemoryLimits

	err := conn.retry(func() error {
		var err error
		resultingLimits, err = conn.Connection.CurrentMemoryLimits(handle)
		return err
	})
	if err != nil {
		return garden.MemoryLimits{}, err
	}

	return resultingLimits, nil
}

func (conn *RetryableConnection) LimitBandwidth(handle string, limits garden.BandwidthLimits) (garden.BandwidthLimits, error) {
	var resultingLimits garden.BandwidthLimits

	err := conn.retry(func() error {
		var err error
		resultingLimits, err = conn.Connection.LimitBandwidth(handle, limits)
		return err
	})
	if err != nil {
		return garden.BandwidthLimits{}, err
	}

	return resultingLimits, nil
}

func (conn *RetryableConnection) LimitCPU(handle string, limits garden.CPULimits) (garden.CPULimits, error) {
	var resultingLimits garden.CPULimits

	err := conn.retry(func() error {
		var err error
		resultingLimits, err = conn.Connection.LimitCPU(handle, limits)
		return err
	})
	if err != nil {
		return garden.CPULimits{}, err
	}

	return resultingLimits, nil
}

func (conn *RetryableConnection) LimitDisk(handle string, limits garden.DiskLimits) (garden.DiskLimits, error) {
	var resultingLimits garden.DiskLimits

	err := conn.retry(func() error {
		var err error
		resultingLimits, err = conn.Connection.LimitDisk(handle, limits)
		return err
	})
	if err != nil {
		return garden.DiskLimits{}, err
	}

	return resultingLimits, nil
}

func (conn *RetryableConnection) LimitMemory(handle string, limits garden.MemoryLimits) (garden.MemoryLimits, error) {
	var resultingLimits garden.MemoryLimits

	err := conn.retry(func() error {
		var err error
		resultingLimits, err = conn.Connection.LimitMemory(handle, limits)
		return err
	})
	if err != nil {
		return garden.MemoryLimits{}, err
	}

	return resultingLimits, nil
}

func (conn *RetryableConnection) Property(handle string, name string) (string, error) {
	var value string

	err := conn.retry(func() error {
		var err error
		value, err = conn.Connection.Property(handle, name)
		return err
	})
	if err != nil {
		return "", err
	}

	return value, nil
}

func (conn *RetryableConnection) SetProperty(handle string, name string, value string) error {
	return conn.retry(func() error {
		return conn.Connection.SetProperty(handle, name, value)
	})
}

func (conn *RetryableConnection) RemoveProperty(handle string, name string) error {
	return conn.retry(func() error {
		return conn.Connection.RemoveProperty(handle, name)
	})
}

func (conn *RetryableConnection) StreamIn(handle string, spec garden.StreamInSpec) error {
	// We don't retry StreamIn because the other end of the connection may have
	// already started reading the body of our request and to send it again would
	// leave things in an unknown state.
	return conn.Connection.StreamIn(handle, spec)
}

func (conn *RetryableConnection) StreamOut(handle string, spec garden.StreamOutSpec) (io.ReadCloser, error) {
	var readCloser io.ReadCloser

	err := conn.retry(func() error {
		var err error
		readCloser, err = conn.Connection.StreamOut(handle, spec)
		return err
	})
	if err != nil {
		return nil, err
	}

	return readCloser, nil
}

func (conn *RetryableConnection) Run(handle string, processSpec garden.ProcessSpec, processIO garden.ProcessIO) (garden.Process, error) {
	var innerProcess garden.Process

	err := conn.retry(func() error {
		var err error
		innerProcess, err = conn.Connection.Run(handle, processSpec, processIO)
		return err
	})
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

func (conn *RetryableConnection) Attach(handle string, processID uint32, processIO garden.ProcessIO) (garden.Process, error) {
	var innerProcess garden.Process

	err := conn.retry(func() error {
		var err error
		innerProcess, err = conn.Connection.Attach(handle, processID, processIO)
		return err
	})
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

func (conn *RetryableConnection) retry(action func() error) error {
	retryLogger := conn.Logger.Session("retry")
	startTime := time.Now()

	var err error

	var failedAttempts uint
	for {
		err = action()
		if err == nil {
			return nil
		}

		if !conn.retryable(err) {
			break
		}

		failedAttempts++

		delay, keepRetrying := conn.RetryPolicy.DelayFor(failedAttempts)
		if !keepRetrying {
			retryLogger.Error("giving-up", errors.New("giving up"), lager.Data{
				"total-failed-attempts": failedAttempts,
				"ran-for":               time.Now().Sub(startTime).String(),
			})

			break
		}

		retryLogger.Info("retrying", lager.Data{
			"failed-attempts": failedAttempts,
			"next-attempt-in": delay.String(),
			"ran-for":         time.Now().Sub(startTime).String(),
		})

		conn.Sleeper.Sleep(delay)

		conn.Connection, err = conn.ConnectionFactory.BuildConnectionFromDB()
		if err != nil {
			break
		}
	}

	return err
}

func (conn *RetryableConnection) retryable(err error) bool {
	if neterr, ok := err.(net.Error); ok {
		if neterr.Temporary() {
			return true
		}
	}

	s := err.Error()
	for _, retryableError := range retryableErrors {
		if strings.HasSuffix(s, retryableError.Error()) {
			return true
		}
	}

	return false
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
