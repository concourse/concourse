package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"code.cloudfoundry.org/garden"
)

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
	var stderr io.Writer = new(bytes.Buffer)

	if logDest != nil {
		stderr = logDest
	}

	procIO := garden.ProcessIO{
		Stdout: stdout,
		Stderr: stderr,
		Stdin:  bytes.NewBuffer(input),
	}

	_, err = c.Run(procSpec, procIO)
	if err != nil {
		err = fmt.Errorf("container run: %w", err)
		return
	}

	err = json.Unmarshal(stdout.Bytes(), output)
	if err != nil {
		err = fmt.Errorf("output unmarshal: %w", err)
		return
	}

	return
}
