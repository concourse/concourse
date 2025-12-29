package ops_test

import (
	"strings"
	"testing"

	"github.com/concourse/concourse/integration/internal/dctest"
	"github.com/concourse/concourse/integration/internal/flytest"
)

func TestUpgradeDowngradeOps(t *testing.T) {
	latestDC := dctest.Init(t, "../docker-compose.yml", "overrides/named.yml", "overrides/latest.yml")
	devDC := dctest.Init(t, "../docker-compose.yml", "overrides/named.yml")

	t.Run("test upgrade", func(t *testing.T) {
		t.Run("deploy latest release version", func(t *testing.T) {
			latestDC.Run(t, "up", "-d")
		})

		fly := flytest.Init(t, latestDC)
		setupUpgradeDowngrade(t, fly)

		t.Run("upgrade to dev", func(t *testing.T) {
			devDC.Run(t, "up", "-d")
		})
		fly = flytest.Init(t, devDC)

		verifyUpgradeDowngrade(t, fly)
	})

	t.Run("test downgrade", func(t *testing.T) {
		t.Run("migrate back down to latest release version", func(t *testing.T) {
			latest := latestDC.Output(t, "run", "web", "migrate", "--supported-db-version")
			latest = strings.TrimRight(latest, "\n")

			devDC.Run(t, "run", "web", "migrate", "--migrate-db-to-version", latest)
		})

		t.Run("downgrade back to latest release version", func(t *testing.T) {
			latestDC.Run(t, "up", "-d")
		})

		fly := flytest.Init(t, latestDC)

		verifyUpgradeDowngrade(t, fly)
	})
}
