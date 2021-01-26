package ops_test

import (
	"testing"

	"github.com/concourse/concourse/integration/dctest"
	"github.com/concourse/concourse/integration/flytest"
	"github.com/stretchr/testify/require"
)

func TestUpgrade(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "../../docker-compose.yml", "overrides/latest.yml")

	t.Run("deploy latest", func(t *testing.T) {
		require.NoError(t, dc.Run("up", "-d"))
	})

	fly := flytest.Init(t, dc)
	setupUpgradeDowngrade(t, fly)

	dc = dctest.Init(t)

	t.Run("upgrade to dev", func(t *testing.T) {
		require.NoError(t, dc.Run("up", "-d"))
	})

	fly = flytest.Init(t, dc)
	verifyUpgradeDowngrade(t, fly)
}
