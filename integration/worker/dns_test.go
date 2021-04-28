package main

import (
	"testing"

	"github.com/concourse/concourse/integration/internal/dctest"
	"github.com/concourse/concourse/integration/internal/flytest"
)

func TestDNSProxyyEnabled(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "../docker-compose.yml", "overrides/enable_dns_proxy.yml")

	t.Run("deploy with DNS proxy enabled", func(t *testing.T) {
		dc.Run(t, "up", "-d")
	})

	fly := flytest.Init(t, dc)
	fly.WithEnv("EXPECTED_EXIT_CODE=0").Run(t, "execute", "-c", "tasks/assert_web_is_reachable.yml")
}

func TestDNSProxyyDisabled(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "../docker-compose.yml", "overrides/disable_dns_proxy.yml")

	t.Run("deploy with DNS proxy disabled", func(t *testing.T) {
		dc.Run(t, "up", "-d")
	})

	fly := flytest.Init(t, dc)
	fly.WithEnv("EXPECTED_EXIT_CODE=1").Run(t, "execute", "-c", "tasks/assert_web_is_reachable.yml")
}

func TestExtraDNSServersAreAdded(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "../docker-compose.yml", "overrides/add_extra_dns_servers.yml")

	t.Run("deploy with extra DNS servers", func(t *testing.T) {
		dc.Run(t, "up", "-d")
	})

	fly := flytest.Init(t, dc)
	fly.WithEnv("EXTRA_SERVER=1.1.1.1").Run(t, "execute", "-c", "tasks/assert_extra_dns_servers.yml")
}
