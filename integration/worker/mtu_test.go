package main

import (
	"testing"

	"github.com/concourse/concourse/integration/internal/dctest"
	"github.com/concourse/concourse/integration/internal/flytest"
	"github.com/stretchr/testify/require"
)

func TestMTU_Infer(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "../docker-compose.yml", "overrides/bridge_mtu.yml")

	t.Run("deploy with MTU set on bridge network", func(t *testing.T) {
		dc.Run(t, "up", "-d")
	})

	fly := flytest.Init(t, dc)
	output := fly.WithEnv("FILE=/sys/class/net/eth0/mtu").Output(t, "execute", "-c", "tasks/cat_file.yml")
	require.Contains(t, output, "1480")
}

func TestMTU_Manual(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "../docker-compose.yml", "overrides/custom_mtu.yml")

	t.Run("deploy with custom MTU", func(t *testing.T) {
		dc.Run(t, "up", "-d")
	})

	fly := flytest.Init(t, dc)
	output := fly.WithEnv("FILE=/sys/class/net/eth0/mtu").Output(t, "execute", "-c", "tasks/cat_file.yml")
	require.Contains(t, output, "1234")
}
