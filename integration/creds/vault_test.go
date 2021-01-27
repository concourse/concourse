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

	dc := dctest.Init(t, "../../docker-compose.yml", "overrides/vault.yml")
	require.NoError(t, dc.Run("up", "-d"))

	vault := vaulttest.Init(t, dc)

	fly := flytest.Init(t, dc)

	// set up kv v1 store for Concourse
	require.NoError(t, vault.Run("secrets", "enable", "-version=1", "-path", "concourse/main", "kv"))

	// set up a policy for Concourse
	require.NoError(t, vault.WithInput(bytes.NewBufferString(`
		path "concourse/*" {
			policy = "read"
		}
	`)).Run("policy", "write", "concourse", "-"))

	// set up cert-based auth
	require.NoError(t, vault.Run("auth", "enable", "cert"))
	require.NoError(t, vault.Write("auth/cert/certs/concourse", map[string]interface{}{
		"policies":    "concourse",
		"certificate": "@/vault/certs/vault-ca.crt", // resolved within container
		"ttl":         "1h",
	}))

	testCredentialManagement(t, fly, dc,
		func(team, key string, val interface{}) {
			path := fmt.Sprintf("concourse/%s/%s", team, key)

			err := vault.Write(path, val)
			require.NoError(t, err)
		},
		func(team, pipeline, key string, val interface{}) {
			path := fmt.Sprintf("concourse/%s/%s/%s", team, pipeline, key)

			err := vault.Write(path, val)
			require.NoError(t, err)
		},
	)
}
