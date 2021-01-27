package ops_test

import (
	"testing"

	"github.com/concourse/concourse/integration/internal/dctest"
	"github.com/concourse/concourse/integration/internal/flytest"
	"github.com/stretchr/testify/require"
)

func TestUpgrade(t *testing.T) {
	t.Parallel()

	latestDC := dctest.Init(t, "../docker-compose.yml", "overrides/latest.yml")

	t.Run("deploy latest", func(t *testing.T) {
		require.NoError(t, latestDC.Run("up", "-d"))
	})

	fly := flytest.Init(t, latestDC)
	setupUpgradeDowngrade(t, fly)

	devDC := dctest.Init(t, "../docker-compose.yml")

	t.Run("upgrade to dev", func(t *testing.T) {
		require.NoError(t, devDC.Run("up", "-d"))
	})

	fly = flytest.Init(t, devDC)
	verifyUpgradeDowngrade(t, fly)
}
