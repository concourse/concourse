package ops_test

import (
	"fmt"
	"testing"

	"github.com/concourse/concourse/integration/dctest"
	"github.com/concourse/concourse/integration/flytest"
	"github.com/concourse/concourse/integration/vaulttest"
	"github.com/stretchr/testify/require"
)

func TestVault(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "overrides/vault.yml")

	require.NoError(t, dc.Run("up", "-d"))

	vault := vaulttest.Init(t, dc)

	// set up kv v1 store for Concourse
	require.NoError(t, vault.Run("secrets", "enable", "-version=1", "-path", "concourse/main", "kv"))

	// set up a policy for Concourse
	require.NoError(t, vault.Run("policy", "write", "concourse", "vault/config/concourse-policy.hcl"))

	// set up cert-based auth
	require.NoError(t, vault.Run("auth", "enable", "cert"))
	require.NoError(t, vault.Write("auth/cert/certs/concourse", map[string]string{
		"policies":    "concourse",
		"certificate": "@vault/certs/vault-ca.crt",
		"ttl":         "1h",
	}))

	fly := flytest.Init(t, dc)

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
