package ops_test

import (
	"strings"
	"testing"

	"github.com/concourse/concourse/integration/internal/dctest"
	"github.com/concourse/concourse/integration/internal/flytest"
)

func TestDowngradeOps(t *testing.T) {
	devDC := dctest.Init(t, "../docker-compose.yml", "overrides/named.yml", "overrides/fast-gc.yml")

	t.Run("deploy dev", func(t *testing.T) {
		devDC.Run(t, "up", "-d")
	})

	fly := flytest.Init(t, devDC)
	setupUpgradeDowngrade(t, fly)
	volumes := fly.Table(t, "volumes")
	beforeVolumes := getColumnValues(volumes, "handle")

	latestDC := dctest.Init(t, "../docker-compose.yml", "overrides/named.yml", "overrides/latest.yml", "overrides/fast-gc.yml")

	t.Run("migrate down to latest from clean deploy", func(t *testing.T) {
		// just to see what it was before
		devDC.Run(t, "run", "web", "migrate", "--current-db-version")

		latest := latestDC.Output(t, "run", "web", "migrate", "--supported-db-version")
		latest = strings.TrimRight(latest, "\n")

		devDC.Run(t, "run", "web", "migrate", "--migrate-db-to-version", latest)
	})

	t.Run("deploy latest", func(t *testing.T) {
		latestDC.Run(t, "up", "-d")
	})

	fly = flytest.Init(t, latestDC)
	waitForVolumesGC(t, fly, beforeVolumes)

	verifyUpgradeDowngrade(t, fly)

	volumes = fly.Table(t, "volumes")
	beforeVolumes = getColumnValues(volumes, "handle")

	t.Run("migrate up to dev and deploy dev", func(t *testing.T) {
		devDC.Run(t, "up", "-d")
	})

	fly = flytest.Init(t, devDC)
	waitForVolumesGC(t, fly, beforeVolumes)

	verifyUpgradeDowngrade(t, fly)
}
