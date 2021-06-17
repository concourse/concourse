package worker_test

import (
	"bytes"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/concourse/concourse/integration/internal/dctest"
	"github.com/concourse/concourse/integration/internal/flytest"
	"github.com/stretchr/testify/require"
)

func TestRestart_AttachToRunningBuild(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "../docker-compose.yml", "overrides/restartable.yml")

	t.Run("deploy", func(t *testing.T) {
		dc.Run(t, "up", "-d")
	})

	fly := flytest.Init(t, dc)
	buf := new(Buffer)
	executeCmd := fly.OutputTo(buf).Start(t, "execute", "-c", "tasks/wait.yml")

	waitForBuildOutput := func(t *testing.T) {
		require.Eventually(t, func() bool {
			return strings.Contains(buf.String(), "waiting for /tmp/stop-waiting to exist")
		}, 1*time.Minute, 1*time.Second)
	}

	waitForBuildOutput(t)

	buildRegex := regexp.MustCompile(`executing build (\d+)`)
	matches := buildRegex.FindStringSubmatch(buf.String())
	buildID := string(matches[1])

	t.Run("restart worker process", func(t *testing.T) {
		// entrypoint script traps SIGHUP and restarts the process
		dc.Run(t, "kill", "-s", "SIGHUP", "worker")

		workerReady := func() bool {
			err := dc.Silence().Try("exec", "-T", "worker", "stat", "/ready")
			return err == nil
		}

		require.Eventually(t, workerReady, 1*time.Minute, 1*time.Second)
	})

	t.Run("task can be re-attached to", func(t *testing.T) {
		// Wait for build logs to appear again
		buf.Reset()
		waitForBuildOutput(t)

		fly.Run(t, "hijack", "-b", buildID, "-s", "one-off", "--", "touch", "/tmp/stop-waiting")

		// assert exits successfully
		err := executeCmd.Wait()
		require.NoError(t, err)

		require.Contains(t, buf.String(), "done")
	})
}

// Thread-safe version of bytes.Buffer
type Buffer struct {
	buffer bytes.Buffer
	mtx    sync.Mutex
}

func (buf *Buffer) Write(p []byte) (n int, err error) {
	buf.mtx.Lock()
	defer buf.mtx.Unlock()
	return buf.buffer.Write(p)
}

func (buf *Buffer) String() string {
	buf.mtx.Lock()
	defer buf.mtx.Unlock()
	return buf.buffer.String()
}

func (buf *Buffer) Reset() {
	buf.mtx.Lock()
	defer buf.mtx.Unlock()
	buf.buffer.Reset()
}
