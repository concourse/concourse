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

func resourceProcessIDProperty(runType string) string {
	return fmt.Sprintf("concourse:%s:resource-process", runType)
}

func resourceResultProperty(runType string) string {
	return fmt.Sprintf("concourse:%s:resource-result", runType)
}

type ErrResourceScriptFailed struct {
	Path       string
	Args       []string
	ExitStatus int

	Stderr string
}

func (err ErrResourceScriptFailed) Error() string {
	msg := fmt.Sprintf(
		"resource script '%s %v' failed: exit status %d",
		err.Path,
		err.Args,
		err.ExitStatus,
	)

	if len(err.Stderr) > 0 {
		msg += "\n\nstderr:\n" + err.Stderr
	}

	return msg
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
	runType string,
) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		if recoverable {
			result, err := resource.container.Property(resourceResultProperty(runType))
			if err == nil {
				return json.Unmarshal([]byte(result), &output)
			}
		}

		request, err := json.Marshal(input)
		if err != nil {
			fmt.Println("failed to marshal", err)
			return err
		}

		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)

		processIO := garden.ProcessIO{
			Stdin:  bytes.NewBuffer(request),
			Stdout: stdout,
		}

		if logDest != nil {
			processIO.Stderr = logDest
		} else {
			processIO.Stderr = stderr
		}

		var process garden.Process

		var processIDProp string
		if recoverable {
			processIDProp, err = resource.container.Property(resourceProcessIDProperty(runType))
			if err != nil {
				processIDProp = ""
			}
		}

		if processIDProp != "" {
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

				err := resource.container.SetProperty(resourceProcessIDProperty(runType), processIDValue)
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
					ExitStatus: status,

					Stderr: stderr.String(),
				}
			}

			if recoverable {
				err := resource.container.SetProperty(resourceResultProperty(runType), stdout.String())
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
