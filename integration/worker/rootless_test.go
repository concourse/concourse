package worker_test

import (
	"testing"

	"github.com/concourse/concourse/v8/integration/internal/dctest"
	"github.com/concourse/concourse/v8/integration/internal/flytest"
)

func TestRootless_PrivilegedMode(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "../docker-compose.yml", "overrides/rootless.yml")
	dc.Run(t, "up", "-d")

	fly := flytest.Init(t, dc)
	fly.Run(t, "set-pipeline", "-n", "-c", "pipelines/rootless.yml", "-p", "test")
	fly.Run(t, "unpause-pipeline", "-p", "test")

	t.Run("joins namespaces and gets the tun device without granting full root", func(t *testing.T) {
		fly.Run(t, "trigger-job", "-j", "test/rootless-namespaces-and-devices", "--watch")
	})
}
