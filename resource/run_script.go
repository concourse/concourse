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

const resourceProcessIDPropertyName = "concourse:resource-process"
const resourceResultPropertyName = "concourse:resource-result"

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
) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		request, err := json.Marshal(input)
		if err != nil {
			return err
		}

		if recoverable {
			result, err := resource.container.Property(resourceResultPropertyName)
			if err == nil {
				return json.Unmarshal([]byte(result), &output)
			}
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

		var processID string
		if recoverable {
			processID, err = resource.container.Property(resourceProcessIDPropertyName)
			if err != nil {
				processID = ""
			}
		}

		if processID != "" {
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

			props, err := resource.container.Properties()
			if err != nil {
				return err
			}
			user := props["user"]
			if user == "" {
				user = "root"
			}

			process, err = resource.container.Run(garden.ProcessSpec{
				Path: path,
				Args: args,
				User: user,
			}, processIO)
			if err != nil {
				return err
			}

			if recoverable {
				err := resource.container.SetProperty(resourceProcessIDPropertyName, process.ID())
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
