package workercmd

import (
	"os"
	"os/exec"
)

type CmdRunner struct {
	Cmd *exec.Cmd
}

func (runner CmdRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	err := runner.Cmd.Start()
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
