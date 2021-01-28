package ops_test

import (
	"testing"

	"github.com/concourse/concourse/integration/internal/flytest"
	"github.com/stretchr/testify/require"
)

func setupUpgradeDowngrade(t *testing.T, fly flytest.Cmd) {
	t.Run("set up test pipeline", func(t *testing.T) {
		err := fly.Run("set-pipeline", "-p", "test", "-c", "pipelines/smoke-pipeline.yml", "-n")
		require.NoError(t, err)

		err = fly.Run("unpause-pipeline", "-p", "test")
		require.NoError(t, err)

		err = fly.Run("trigger-job", "-j", "test/say-hello", "-w")
		require.NoError(t, err)
	})
}

func verifyUpgradeDowngrade(t *testing.T, fly flytest.Cmd) {
	t.Run("pipeline and build still exists", func(t *testing.T) {
		err := fly.Run("get-pipeline", "-p", "test")
		require.NoError(t, err)

		out, err := fly.Output("watch", "-j", "test/say-hello", "--ignore-event-parsing-errors")
		require.NoError(t, err)
		require.Contains(t, out, "Hello, world!")
	})

	t.Run("can still run pipeline builds", func(t *testing.T) {
		err := fly.Run("trigger-job", "-j", "test/say-hello", "-w")
		require.NoError(t, err)
	})

	t.Run("can still run checks", func(t *testing.T) {
		err := fly.Run("check-resource", "-r", "test/mockery")
		require.NoError(t, err)
	})

	t.Run("can still run one-off builds", func(t *testing.T) {
		out, err := fly.Output("execute", "-c", "tasks/hello.yml")
		require.NoError(t, err)
		require.Contains(t, out, "hello")
	})
}
