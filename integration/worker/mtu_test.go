package main

import (
	"testing"

	"github.com/concourse/concourse/integration/internal/dctest"
	"github.com/concourse/concourse/integration/internal/flytest"
)

func TestMTU_Infer(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "../docker-compose.yml", "overrides/bridge_mtu.yml")

	t.Run("deploy with MTU set on bridge network", func(t *testing.T) {
		dc.Run(t, "up", "-d")
	})

	fly := flytest.Init(t, dc)
	fly.WithEnv("EXPECTED_MTU=1480").Run(t, "execute", "-c", "tasks/assert_mtu.yml")
}

func TestMTU_Manual(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "../docker-compose.yml", "overrides/custom_mtu.yml")

	t.Run("deploy with custom MTU", func(t *testing.T) {
		dc.Run(t, "up", "-d")
	})

	fly := flytest.Init(t, dc)
	fly.WithEnv("EXPECTED_MTU=1234").Run(t, "execute", "-c", "tasks/assert_mtu.yml")
}
