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

	devDC := dctest.Init(t, "../docker-compose.yml")

	t.Run("deploy dev", func(t *testing.T) {
		require.NoError(t, devDC.Run("up", "-d"))
	})

	fly := flytest.Init(t, devDC)
	setupUpgradeDowngrade(t, fly)

	latestDC := dctest.Init(t, "../docker-compose.yml", "overrides/latest.yml")

	t.Run("migrate down to latest from clean deploy", func(t *testing.T) {
		// just to see what it was before
		err := devDC.Run("run", "web", "migrate", "--current-db-version")
		require.NoError(t, err)

		latest, err := latestDC.Output("run", "web", "migrate", "--supported-db-version")
		require.NoError(t, err)
		latest = strings.TrimRight(latest, "\n")

		err = devDC.Run("run", "web", "migrate", "--migrate-db-to-version", latest)
		require.NoError(t, err)
	})

	t.Run("deploy latest", func(t *testing.T) {
		require.NoError(t, latestDC.Run("up", "-d"))
	})

	fly = flytest.Init(t, latestDC)
	verifyUpgradeDowngrade(t, fly)

	t.Run("migrate up to dev and deploy dev", func(t *testing.T) {
		require.NoError(t, devDC.Run("up", "-d"))
	})

	fly = flytest.Init(t, devDC)
	verifyUpgradeDowngrade(t, fly)
}
