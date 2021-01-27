package ops_test

import (
	"strings"
	"testing"

	"github.com/concourse/concourse/integration/internal/dctest"
	"github.com/concourse/concourse/integration/internal/flytest"
	"github.com/stretchr/testify/require"
)

func TestDowngrade(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "../../docker-compose.yml")

	t.Run("deploy dev", func(t *testing.T) {
		require.NoError(t, dc.Run("up", "-d"))
	})

	fly := flytest.Init(t, dc)
	setupUpgradeDowngrade(t, fly)

	latestDC := dctest.Init(t, "../../docker-compose.yml", "overrides/latest.yml")

	latest, err := latestDC.Output("run", "web", "migrate", "--supported-db-version")
	require.NoError(t, err)
	latest = strings.TrimRight(latest, "\n")

	t.Run("down migrations", func(t *testing.T) {
		// just to see what it was before
		err := dc.Run("run", "web", "migrate", "--current-db-version")
		require.NoError(t, err)

		err = dc.Run("run", "web", "migrate", "--migrate-db-to-version", latest)
		require.NoError(t, err)
	})

	t.Run("downgrade to latest", func(t *testing.T) {
		require.NoError(t, latestDC.Run("up", "-d"))
	})

	fly = flytest.Init(t, latestDC)
	verifyUpgradeDowngrade(t, fly)

	t.Run("upgrading after downgrade", func(t *testing.T) {
		require.NoError(t, dc.Run("up", "-d"))
	})

	fly = flytest.Init(t, dc)
	verifyUpgradeDowngrade(t, fly)
}

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

		out, err := fly.Output("watch", "-j", "test/say-hello")
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
