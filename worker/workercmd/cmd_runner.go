package workercmd

import (
	"errors"
	"os"
	"os/exec"
	"time"
)

type CmdRunner struct {
	Cmd *exec.Cmd
	// Logic to validate that the command, e.g. a daemon, has executed successfully
	Ready func() bool
	// Maximum period to wait for Ready() to return true
	Timeout time.Duration
}

func (runner CmdRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	err := runner.Cmd.Start()
	if err != nil {
		return err
	}

	err = runner.waitUntilReady()
	if err != nil {
		return err
	}

	close(ready)

	waitErr := make(chan error, 1)

	go func() {
		waitErr <- runner.Cmd.Wait()
	}()

	for {
		select {
		case sig := <-signals:
			runner.Cmd.Process.Signal(sig)
		case err := <-waitErr:
			return err
		}
	}
}

func (runner CmdRunner) waitUntilReady() error {
	timeout := time.After(runner.Timeout)
	for {
		select {
		case <-timeout:
			return errors.New("timed out trying to start process")
		default:
			if runner.Ready() {
				return nil
			}
			time.Sleep(time.Second)
		}
	}
}
