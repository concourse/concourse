package ops_test

import (
	"testing"

	"github.com/concourse/concourse/integration/internal/dctest"
	"github.com/concourse/concourse/integration/internal/flytest"
)

func TestUpgradeOps(t *testing.T) {
	latestDC := dctest.Init(t, "../docker-compose.yml", "overrides/named.yml", "overrides/latest.yml", "overrides/fast-gc.yml")

	t.Run("deploy latest", func(t *testing.T) {
		latestDC.Run(t, "up", "-d")
	})

	fly := flytest.Init(t, latestDC)
	setupUpgradeDowngrade(t, fly)

	volumes := fly.Table(t, "volumes")
	beforeVolumes := getColumnValues(volumes, "handle")

	devDC := dctest.Init(t, "../docker-compose.yml", "overrides/named.yml", "overrides/fast-gc.yml")

	t.Run("upgrade to dev", func(t *testing.T) {
		devDC.Run(t, "up", "-d")
	})
	fly = flytest.Init(t, devDC)
	waitForVolumesGC(t, fly, beforeVolumes)

	verifyUpgradeDowngrade(t, fly)
}
