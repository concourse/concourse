package vaulttest

import (
	"fmt"
	"testing"
	"time"

	"github.com/concourse/concourse/integration/internal/cmdtest"
	"github.com/concourse/concourse/integration/internal/dctest"
	"github.com/stretchr/testify/require"
)

type Cmd struct {
	cmdtest.Cmd
}

func Init(t *testing.T, dc dctest.Cmd) Cmd {
	vault, _ := dc.WithArgs("exec", "-T", "vault", "vault").WithTempHome(t)

	var initOut struct {
		UnsealKeys []string `json:"unseal_keys_b64"`
		RootToken  string   `json:"root_token"`
	}

	var err error
	for i := 0; i < 5; i++ {
		err = vault.TryOutputJSON(&initOut, "operator", "init")
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		vault.Run(t, "operator", "unseal", initOut.UnsealKeys[i])
	}

	// log in with root token
	vault.Run(t, "login", initOut.RootToken)

	return Cmd{vault}
}

func (cmd Cmd) Write(t *testing.T, path string, val any) {
	cmd.WithArgs("write", path).Run(t, writeArgs(val)...)
}

func writeArgs(val any) []string {
	vals := []string{}
	switch x := val.(type) {
	case map[string]any:
		for k, v := range x {
			vals = append(vals, fmt.Sprintf("%s=%s", k, v))
		}
	default:
		vals = append(vals, fmt.Sprintf("value=%v", x))
	}

	return vals
}
