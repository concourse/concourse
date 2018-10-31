package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/atc/worker"
)

const resourceResultPropertyName = "concourse:resource-result"

const TaskProcessID = "resource"

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

func RunScript(
	ctx context.Context,
	path string,
	args []string,
	input interface{},
	output interface{},
	logDest io.Writer,
	recoverable bool,
	container worker.Container,
) error {
	request, err := json.Marshal(input)
	if err != nil {
		return err
	}

	if recoverable {
		result, err := container.Property(resourceResultPropertyName)
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

	if recoverable {
		process, err = container.Attach(TaskProcessID, processIO)
		if err != nil {
			process, err = container.Run(garden.ProcessSpec{
				ID:   TaskProcessID,
				Path: path,
				Args: args,
			}, processIO)
			if err != nil {
				return err
			}
		}
	} else {
		process, err = container.Run(garden.ProcessSpec{
			Path: path,
			Args: args,
		}, processIO)
		if err != nil {
			return err
		}
	}

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
			return ErrResourceScriptFailed{
				Path:       path,
				Args:       args,
				ExitStatus: processStatus,

				Stderr: stderr.String(),
			}
		}

		if recoverable {
			err := container.SetProperty(resourceResultPropertyName, stdout.String())
			if err != nil {
				return err
			}
		}

		return json.Unmarshal(stdout.Bytes(), output)

	case <-ctx.Done():
		container.Stop(false)
		<-processExited
		return ctx.Err()
	}
}
