package worker_test

import (
	"strings"
	"testing"
	"time"

	"github.com/concourse/concourse/integration/internal/dctest"
	"github.com/stretchr/testify/require"
)

func TestRegistration_GardenServerFails(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "../docker-compose.yml", "overrides/broken_containerd_socket.yml")

	t.Run("deploy with a broken containerd socket", func(t *testing.T) {
		dc.Run(t, "up", "-d")
	})

	t.Run("worker container exits", func(t *testing.T) {
		require.Eventually(t, func() bool {
			services := dc.Output(t, "ps", "-a", "--services", "--status=exited")
			return strings.Contains(services, "worker")
		}, 30*time.Second, 1*time.Second, "worker process never exited, but should have")
	})
}
