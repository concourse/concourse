package creds_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/concourse/concourse/integration/internal/dctest"
	"github.com/concourse/concourse/integration/internal/flytest"
	"github.com/concourse/concourse/integration/internal/vaulttest"
	"github.com/stretchr/testify/require"
)

func TestVault(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "../docker-compose.yml", "overrides/vault.yml")
	dc.Run(t, "up", "-d")

	vault := vaulttest.Init(t, dc)

	fly := flytest.Init(t, dc)

	// set up kv v1 store for Concourse
	vault.Run(t, "secrets", "enable", "-version=1", "-path", "concourse/main", "kv")

	setupVaultAuth(t, vault)

	testCredentialManagement(t, fly, dc,
		func(team, key string, val interface{}) {
			vault.Write(t,
				fmt.Sprintf("concourse/%s/%s", team, key),
				val,
			)
		},
		func(team, pipeline, key string, val interface{}) {
			vault.Write(t,
				fmt.Sprintf("concourse/%s/%s/%s", team, pipeline, key),
				val,
			)
		},
	)
}

func TestVaultV2WithUnmountPath(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "../docker-compose.yml", "overrides/vault.yml")
	dc.Run(t, "up", "-d")

	vault := vaulttest.Init(t, dc)

	fly := flytest.Init(t, dc)

	// set up kv v2 store for Concourse. path is set to concourse/main so that shared path doesn't exist.
	vault.Run(t, "secrets", "enable", "-version=2", "-path", "concourse/main", "kv")

	setupVaultAuth(t, vault)

	result := fly.ExpectExit(2).Output(t, "execute", "-c", "tasks/basic.yml")

	require.NotContains(t, result, "403")
}

func setupVaultAuth(t *testing.T, vault vaulttest.Cmd) {
	// set up a policy for Concourse
	vault.WithInput(bytes.NewBufferString(`
		path "concourse/*" {
			policy = "read"
		}
	`)).Run(t, "policy", "write", "concourse", "-")

	// set up cert-based auth
	vault.Run(t, "auth", "enable", "cert")
	vault.Write(t, "auth/cert/certs/concourse", map[string]interface{}{
		"policies":    "concourse",
		"certificate": "@/vault/certs/vault-ca.crt", // resolved within container
		"ttl":         "1h",
	})
}
