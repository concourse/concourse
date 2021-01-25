package ops_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDowngrade(t *testing.T) {
	t.Parallel()

	dc := dockerCompose(t)

	t.Run("deploy dev", func(t *testing.T) {
		require.NoError(t, dc.Run("up", "-d"))
	})

	fly := initFly(t, dc)

	t.Run("set up test pipeline", func(t *testing.T) {
		err := fly.Run("set-pipeline", "-p", "test", "-c", "pipelines/smoke-pipeline.yml", "-n")
		require.NoError(t, err)

		err = fly.Run("unpause-pipeline", "-p", "test")
		require.NoError(t, err)

		err = fly.Run("trigger-job", "-j", "test/say-hello", "-w")
		require.NoError(t, err)
	})

	latestDC := dockerCompose(t, "overrides/latest.yml")

	latest, err := latestDC.Output("run", "web", "migrate", "--supported-db-version")
	require.NoError(t, err)
	latest = strings.TrimRight(latest, "\n")

	t.Run("downgrading", func(t *testing.T) {
		// just to see what it was before
		err := dc.Run("run", "web", "migrate", "--current-db-version")
		require.NoError(t, err)

		err = dc.Run("run", "web", "migrate", "--migrate-db-to-version", latest)
		require.NoError(t, err)

		require.NoError(t, latestDC.Run("up", "-d"))
	})

	fly = initFly(t, latestDC)

	t.Run("pipeline and build still exists", func(t *testing.T) {
		err := fly.Run("get-pipeline", "-p", "test")
		require.NoError(t, err)

		out, err := fly.Output("watch", "-j", "test/say-hello", "-b", "1")
		require.NoError(t, err)
		require.Contains(t, out, "Hello, world!")
	})

	t.Run("can still run pipeline builds", func(t *testing.T) {
		err := fly.Run("trigger-job", "-j", "test/say-hello", "-w")
		require.NoError(t, err)
	})

	t.Run("can still run checks", func(t *testing.T) {
		err = fly.Run("check-resource", "-r", "test/mockery")
		require.NoError(t, err)
	})

	t.Run("can still reach the internet", func(t *testing.T) {
		out, err := fly.Output("trigger-job", "-j", "test/use-the-internet", "-w")
		require.NoError(t, err)
		require.Contains(t, out, "Example Domain")
	})

	t.Run("can still run one-off builds", func(t *testing.T) {
		out, err := fly.Output("execute", "-c", "tasks/hello.yml")
		require.NoError(t, err)
		require.Contains(t, out, "hello")
	})
}
