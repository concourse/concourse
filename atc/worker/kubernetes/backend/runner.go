package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/atc/runtime"
)

// TODO handle recoverable
//
func (c *Container) RunScript(
	ctx context.Context,
	path string,
	args []string,
	input []byte,
	output interface{},
	logDest io.Writer,
	recoverable bool,
) (err error) {
	procSpec := garden.ProcessSpec{
		Path: path,
		Args: args,
	}

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	procIO := garden.ProcessIO{
		Stdout: stdout,
		Stderr: stderr,
		Stdin:  bytes.NewBuffer(input),
	}

	if logDest != nil {
		procIO.Stderr = logDest
	}

	exitStatus, err := c.Run(procSpec, procIO)
	if err != nil {
		err = fmt.Errorf("container run: %w", err)
		return
	}
	if exitStatus != 0 {
		err = runtime.ErrResourceScriptFailed{
			Path:       path,
			Args:       args,
			ExitStatus: exitStatus,
			Stderr:     stderr.String(),
		}
	}

	err = json.Unmarshal(stdout.Bytes(), output)
	if err != nil {
		err = fmt.Errorf("output unmarshal: %w", err)
		return
	}

	return
}
