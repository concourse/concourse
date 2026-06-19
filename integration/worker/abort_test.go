package worker_test

import (
	"strings"
	"testing"
	"time"

	"github.com/concourse/concourse/integration/internal/dctest"
	"github.com/concourse/concourse/integration/internal/flytest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAbort_WorkerDisappears(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "../docker-compose.yml")
	dc.Run(t, "up", "-d")

	fly := flytest.Init(t, dc)
	fly.Run(t, "set-pipeline", "-n", "-c", "pipelines/wait.yml", "-p", "test")
	fly.Run(t, "unpause-pipeline", "-p", "test")

	buf := new(Buffer)
	tjCmd := fly.OutputTo(buf).Start(t, "trigger-job", "-j", "test/wait", "--watch")
	require.Eventually(t, func() bool {
		return strings.Contains(buf.buffer.String(), "waiting for /tmp/stop-waiting to exist")
	}, 1*time.Minute, 1*time.Second)

	dc.Run(t, "rm", "--stop", "worker")

	fly.Run(t, "abort-build", "-j", "test/wait", "-b", "1")

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.Contains(c, buf.String(), "interrupted")
	}, 1*time.Minute, 1*time.Second)

	err := tjCmd.Wait()
	require.Error(t, err)
}
