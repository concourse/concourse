package main

import (
	"os"
	"os/exec"
)

type cmdRunner struct {
	cmd *exec.Cmd
}

func (runner cmdRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	err := runner.cmd.Start()
	if err != nil {
		return err
	}

	close(ready)

	waitErr := make(chan error, 1)

	go func() {
		waitErr <- runner.cmd.Wait()
	}()

	for {
		select {
		case sig := <-signals:
			runner.cmd.Process.Signal(sig)
		case err := <-waitErr:
			return err
		}
	}
}
