package vaulttest

import (
	"fmt"
	"testing"

	"github.com/concourse/concourse/integration/cmdtest"
	"github.com/concourse/concourse/integration/dctest"
	"github.com/stretchr/testify/require"
)

type Cmd struct {
	cmdtest.Cmd
}

func Init(t *testing.T, dc dctest.Cmd) Cmd {
	vault := dc.WithArgs("exec", "-T", "vault", "vault").WithTempHome(t)

	var initOut struct {
		UnsealKeys []string `json:"unseal_keys_b64"`
		RootToken  string   `json:"root_token"`
	}

	err := vault.OutputJSON(&initOut, "operator", "init")
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		err := vault.Run("operator", "unseal", initOut.UnsealKeys[i])
		require.NoError(t, err)
	}

	// log in with root token
	require.NoError(t, vault.Run("login", initOut.RootToken))

	return Cmd{vault}
}

func (cmd Cmd) Write(path string, val interface{}) error {
	return cmd.WithArgs("write", path).Run(writeArgs(val)...)
}

func writeArgs(val interface{}) []string {
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
