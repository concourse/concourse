package main

import (
	"testing"

	"github.com/concourse/concourse/integration/internal/dctest"
	"github.com/concourse/concourse/integration/internal/flytest"
	"github.com/stretchr/testify/require"
)

func TestDNSProxyEnabled(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "../docker-compose.yml", "overrides/enable_dns_proxy.yml")

	t.Run("deploy with DNS proxy enabled", func(t *testing.T) {
		dc.Run(t, "up", "-d")
	})

	fly := flytest.Init(t, dc)
	fly.ExpectExit(0).Run(t, "execute", "-c", "tasks/resolve_web_dns.yml")
}

func TestDNSProxyDisabled(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "../docker-compose.yml", "overrides/disable_dns_proxy.yml")

	t.Run("deploy with DNS proxy disabled", func(t *testing.T) {
		dc.Run(t, "up", "-d")
	})

	fly := flytest.Init(t, dc)
	fly.ExpectExit(1).Run(t, "execute", "-c", "tasks/resolve_web_dns.yml")
}

func TestExtraDNSServersAreAdded(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "../docker-compose.yml", "overrides/add_extra_dns_servers.yml")

	t.Run("deploy with extra DNS servers", func(t *testing.T) {
		dc.Run(t, "up", "-d")
	})

	fly := flytest.Init(t, dc)
	output := fly.WithEnv("FILE=/etc/resolv.conf").Output(t, "execute", "-c", "tasks/cat_file.yml")
	require.Contains(t, output, "nameserver 1.1.1.1")
}
