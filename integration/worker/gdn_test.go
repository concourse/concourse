package worker_test

import (
	"runtime"
	"testing"

	"github.com/concourse/concourse/integration/internal/dctest"
	"github.com/concourse/concourse/integration/internal/flytest"
	"github.com/stretchr/testify/require"
)

func TestGuardianConfig_ConfigFile(t *testing.T) {
	t.Parallel()

	if runtime.GOARCH == "arm" {
		// https://github.com/cloudfoundry/garden-runc-release/issues/378
		t.Skip("guardian doesn't work on arm64")
	}

	dc := dctest.Init(t, "../docker-compose.yml", "overrides/guardian.yml", "overrides/garden_config.yml")

	t.Run("deploy with max containers configured in garden config file", func(t *testing.T) {
		dc.Run(t, "up", "-d")
		// Wait for worker to come up
		flytest.Init(t, dc)
	})

	require.Equal(t, 100, getMaxContainers(t, dc))
}

func TestGuardianConfig_EnvVars(t *testing.T) {
	t.Parallel()

	if runtime.GOARCH == "arm" {
		// https://github.com/cloudfoundry/garden-runc-release/issues/378
		t.Skip("guardian doesn't work on arm64")
	}

	dc := dctest.Init(t, "../docker-compose.yml", "overrides/guardian.yml", "overrides/garden_max_containers.yml")

	t.Run("deploy with max containers configured via env var", func(t *testing.T) {
		dc.Run(t, "up", "-d")
		// Wait for worker to come up
		flytest.Init(t, dc)
	})

	require.Equal(t, 100, getMaxContainers(t, dc))
}

func getMaxContainers(t *testing.T, dc dctest.Cmd) int {
	var gardenCap struct {
		MaxContainers int `json:"max_containers"`
	}
	dc.OutputJSON(t, &gardenCap, "exec", "-T", "worker", "curl", "http://localhost:7777/capacity")
	return gardenCap.MaxContainers
}
