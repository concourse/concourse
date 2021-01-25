package ops_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpgrade(t *testing.T) {
	t.Parallel()

	dc, err := dockerCompose(t, "overrides/latest.yml")
	require.NoError(t, err)

	t.Run("deploy latest", func(t *testing.T) {
		require.NoError(t, dc.Run("up", "-d"))
	})

	fly := initFly(t, dc)

	t.Run("set up test pipeline", func(t *testing.T) {
		err = fly.Run("set-pipeline", "-p", "test", "-c", "pipelines/smoke-pipeline.yml", "-n")
		require.NoError(t, err)

		err = fly.Run("unpause-pipeline", "-p", "test")
		require.NoError(t, err)

		err = fly.Run("trigger-job", "-j", "test/say-hello", "-w")
		require.NoError(t, err)
	})

	dc, err = dockerCompose(t)
	require.NoError(t, err)

	t.Run("upgrade to dev", func(t *testing.T) {
		require.NoError(t, dc.Run("up", "-d"))
	})

	fly = initFly(t, dc)

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
