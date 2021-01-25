package ops_test

import (
	"fmt"
	"testing"

	"github.com/concourse/concourse/integration/cmdtest"
	"github.com/stretchr/testify/require"
)

func TestVault(t *testing.T) {
	t.Parallel()

	dc, err := dockerCompose(t, "overrides/vault.yml")
	require.NoError(t, err)

	require.NoError(t, dc.Run("up", "-d"))

	vault := initVault(t, dc)

	fly := initFly(t, dc)

	testCredentialManagement(t, fly, dc,
		func(team, key string, val interface{}) {
			path := fmt.Sprintf("concourse/%s/%s", team, key)
			err := vault.WithArgs("write", path).Run(vaultWriteArgs(val)...)
			require.NoError(t, err)
		},
		func(team, pipeline, key string, val interface{}) {
			path := fmt.Sprintf("concourse/%s/%s/%s", team, pipeline, key)
			err := vault.WithArgs("write", path).Run(vaultWriteArgs(val)...)
			require.NoError(t, err)
		},
	)
}

func vaultWriteArgs(val interface{}) []string {
	vals := []string{}
	switch x := val.(type) {
	case map[string]interface{}:
		for k, v := range x {
			vals = append(vals, fmt.Sprintf("%s=%s", k, v))
		}
	default:
		vals = append(vals, fmt.Sprintf("value=%v", x))
	}

	return vals
}

func initVault(t *testing.T, dc cmdtest.Cmd) cmdtest.Cmd {
	init := dc.WithArgs("exec", "-T", "vault", "vault")

	var initOut struct {
		UnsealKeys []string `json:"unseal_keys_b64"`
		RootToken  string   `json:"root_token"`
	}

	err := init.OutputJSON(&initOut, "operator", "init")
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		err := init.Run("operator", "unseal", initOut.UnsealKeys[i])
		require.NoError(t, err)
	}

	vault := dc.WithArgs(
		"exec",
		"--env", "VAULT_TOKEN="+initOut.RootToken,
		"-T",    // do not use a TTY
		"vault", // service
		"vault", // command
	)

	// set up kv v1 store for Concourse
	require.NoError(t, vault.Run("secrets", "enable", "-version=1", "-path", "concourse/main", "kv"))

	// set up a policy for Concourse
	require.NoError(t, vault.Run("policy", "write", "concourse", "vault/config/concourse-policy.hcl"))

	// set up cert-based auth
	require.NoError(t, vault.Run("auth", "enable", "cert"))
	require.NoError(t, vault.Run(
		"write", "auth/cert/certs/concourse",
		"policies=concourse",
		"certificate=@vault/certs/vault-ca.crt",
		"ttl=1h",
	))

	return vault
}
