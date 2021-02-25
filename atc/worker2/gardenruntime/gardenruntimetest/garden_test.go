package gardenruntimetest

import (
	"bytes"
	"context"
	"testing"

	"code.cloudfoundry.org/garden"
	"github.com/stretchr/testify/require"
)

func TestContainer_Run_Attach(t *testing.T) {
	container := NewContainer("container")

	ctx := context.Background()
	runBuf := new(bytes.Buffer)
	runProc, err := container.Run(ctx, garden.ProcessSpec{
		ID:   "proc",
		Path: "sleep-and-echo",
		Args: []string{"200ms", "hello", "world"},
	}, garden.ProcessIO{
		Stdout: runBuf,
	})
	require.NoError(t, err)

	attachBuf := new(bytes.Buffer)
	attachProc, err := container.Attach(ctx, "proc", garden.ProcessIO{
		Stdout: attachBuf,
	})
	require.NoError(t, err)

	runExitCode, err := runProc.Wait()
	require.NoError(t, err)
	require.Equal(t, 0, runExitCode)

	attachExitCode, err := attachProc.Wait()
	require.NoError(t, err)
	require.Equal(t, 0, attachExitCode)

	require.Equal(t, "hello world\n", runBuf.String())
	require.Equal(t, "hello world\n", attachBuf.String())
}
