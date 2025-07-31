package ops_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/concourse/concourse/integration/internal/dctest"
	"github.com/concourse/concourse/integration/internal/flytest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRollingRestartsOfWebNodes(t *testing.T) {
	dc := dctest.Init(t, "../docker-compose.yml", "overrides/ha-web-nodes.yml")
	dc.Run(t, "up", "-d")

	fly := flytest.Init(t, dc)

	fly.Run(t, "set-pipeline", "-n", "-c", "pipelines/stream-logs.yml", "-p", "test")
	fly.Run(t, "unpause-pipeline", "-p", "test")
	fly.Run(t, "trigger-job", "--job", "test/stream-logs")

	buildLogs := new(bytes.Buffer)
	t.Run("check logs and restart web nodes", func(t *testing.T) {
		t.Parallel()
		t.Run("build logs continue to stream", func(t *testing.T) {
			t.Parallel()
			fly.Stdout = buildLogs
			t.Run("run fly in parallel", func(t *testing.T) {
				t.Parallel()
				fly.Run(t, "watch", "--job", "test/stream-logs")
			})
			t.Run("check logs", func(t *testing.T) {
				t.Parallel()
				require.EventuallyWithT(t, func(collect *assert.CollectT) {
					output := buildLogs.String()
					require.Contains(collect, output, "Hello 1")
					require.Contains(collect, output, "Hello 2")
					require.Contains(collect, output, "Hello 3")
					require.Contains(collect, output, "Hello 110")
					require.Contains(collect, output, "Hello 111")
					require.Contains(collect, output, "Hello 112")
				}, 130*time.Second, 1*time.Second)
			})
		})

		t.Run("Web nodes are restarted", func(t *testing.T) {
			t.Parallel()
			require.EventuallyWithT(t, func(collect *assert.CollectT) {
				output := buildLogs.String()
				require.Contains(collect, output, "Hello 1")
				require.Contains(collect, output, "Hello 2")
				require.Contains(collect, output, "Hello 3")
			}, 60*time.Second, 1*time.Second)

			dc.Run(t, "restart", "web-1")
			time.Sleep(10 * time.Second)
			dc.Run(t, "restart", "web-2")
			time.Sleep(10 * time.Second)
			dc.Run(t, "restart", "web-1")
			time.Sleep(10 * time.Second)
			dc.Run(t, "restart", "web-2")
		})
	})
}
