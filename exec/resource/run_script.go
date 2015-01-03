package resource

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	garden "github.com/cloudfoundry-incubator/garden/api"
	"github.com/tedsuo/ifrit"
)

var ErrAborted = errors.New("script aborted")

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

func (resource *resource) runScript(path string, args []string, input interface{}, output interface{}, logDest io.Writer) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		request, err := json.Marshal(input)
		if err != nil {
			return err
		}

		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)

		var stderrW io.Writer
		if logDest != nil {
			stderrW = io.MultiWriter(logDest, stderr)
		} else {
			stderrW = stderr
		}

		process, err := resource.container.Run(garden.ProcessSpec{
			Path:       path,
			Args:       args,
			Privileged: true,
		}, garden.ProcessIO{
			Stdin:  bytes.NewBuffer(request),
			Stderr: stderrW,
			Stdout: stdout,
		})
		if err != nil {
			return err
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

			return json.Unmarshal(stdout.Bytes(), output)

		case err := <-errCh:
			return err

		case <-signals:
			resource.container.Stop(false)
			return ErrAborted
		}
	})
}
