package resource

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/tedsuo/ifrit"
)

var ErrAborted = errors.New("script aborted")

const resourceProcessIDPropertyName = "resource-process"
const resourceResultPropertyName = "resource-result"

type ErrResourceScriptFailed struct {
	Path       string
	Args       []string
	Stdout     string
	Stderr     string
	ExitStatus int
}

func (err ErrResourceScriptFailed) Error() string {
	return fmt.Sprintf(
		"resource script '%s %v' failed: exit status %d\n\nstdout:\n\n%s\n\nstderr:\n\n%s",
		err.Path,
		err.Args,
		err.ExitStatus,
		err.Stdout,
		err.Stderr,
	)
}

func (resource *resource) runScript(
	path string,
	args []string,
	input interface{},
	output interface{},
	logDest io.Writer,
	inputSource ArtifactSource,
	inputDestination ArtifactDestination,
	recoverable bool,
) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		request, err := json.Marshal(input)
		if err != nil {
			return err
		}

		if recoverable {
			result, err := resource.container.GetProperty(resourceResultPropertyName)
			if err == nil {
				return json.Unmarshal([]byte(result), &output)
			}
		}

		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)

		var stderrW io.Writer
		if logDest != nil {
			stderrW = io.MultiWriter(logDest, stderr)
		} else {
			stderrW = stderr
		}

		processIO := garden.ProcessIO{
			Stdin:  bytes.NewBuffer(request),
			Stderr: stderrW,
			Stdout: stdout,
		}

		var process garden.Process

		processIDProp, err := resource.container.GetProperty(resourceProcessIDPropertyName)
		if recoverable && err == nil {
			var processID uint32
			_, err = fmt.Sscanf(processIDProp, "%d", &processID)
			if err != nil {
				return err
			}

			process, err = resource.container.Attach(processID, processIO)
			if err != nil {
				return err
			}
		} else {
			if inputSource != nil {
				err := inputSource.StreamTo(inputDestination)
				if err != nil {
					return err
				}
			}

			process, err = resource.container.Run(garden.ProcessSpec{
				Path:       path,
				Args:       args,
				Privileged: true,
			}, processIO)
			if err != nil {
				return err
			}

			if recoverable {
				processIDValue := fmt.Sprintf("%d", process.ID())

				err := resource.container.SetProperty(resourceProcessIDPropertyName, processIDValue)
				if err != nil {
					return err
				}
			}
		}

		close(ready)

		statusCh := make(chan int, 1)
		errCh := make(chan error, 1)

		go func() {
			status, err := process.Wait()
			if err != nil {
				errCh <- err
			} else {
				statusCh <- status
			}
		}()

		select {
		case status := <-statusCh:
			if status != 0 {
				return ErrResourceScriptFailed{
					Path:       path,
					Args:       args,
					Stdout:     stdout.String(),
					Stderr:     stderr.String(),
					ExitStatus: status,
				}
			}

			if recoverable {
				err := resource.container.SetProperty(resourceResultPropertyName, stdout.String())
				if err != nil {
					return err
				}
			}

			return json.Unmarshal(stdout.Bytes(), output)

		case err := <-errCh:
			return err

		case <-signals:
			resource.container.Stop(false)
			return ErrAborted
		}
	})
}
