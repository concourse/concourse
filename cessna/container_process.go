package cessna

import (
	"errors"
	"fmt"
	"os"

	"code.cloudfoundry.org/garden"
)

var ErrAborted = errors.New("script aborted")

type ErrScriptFailed struct {
	Path       string
	Args       []string
	ExitStatus int

	Stderr string
	Stdout string
}

func (err ErrScriptFailed) Error() string {
	msg := fmt.Sprintf(
		"script '%s %v' failed: exit status %d",
		err.Path,
		err.Args,
		err.ExitStatus,
	)

	if len(err.Stderr) > 0 {
		msg += "\n\nstderr:\n" + err.Stderr
	}

	if len(err.Stdout) > 0 {
		msg += "\n\nstdout:\n" + err.Stdout
	}

	return msg
}

type ContainerProcess struct {
	Container garden.Container

	ProcessSpec garden.ProcessSpec
	ProcessIO   garden.ProcessIO
}

func (c *ContainerProcess) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	var process garden.Process

	process, err := c.Container.Run(c.ProcessSpec, c.ProcessIO)
	if err != nil {
		return err
	}
	close(ready)

	processExited := make(chan struct{})

	var processStatus int
	var processErr error

	go func() {
		processStatus, processErr = process.Wait()
		close(processExited)
	}()

	select {
	case <-processExited:
		if processErr != nil {
			return processErr
		}

		if processStatus != 0 {
			return ErrScriptFailed{
				Path:       c.ProcessSpec.Path,
				Args:       c.ProcessSpec.Args,
				ExitStatus: processStatus,
			}
		}

		return nil

	case <-signals:
		c.Container.Stop(false)
		<-processExited
		return ErrAborted
	}

	return nil
}
