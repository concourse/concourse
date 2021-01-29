package vaulttest

import (
	"fmt"
	"testing"

	"github.com/concourse/concourse/integration/internal/cmdtest"
	"github.com/concourse/concourse/integration/internal/dctest"
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

	vault.OutputJSON(t, &initOut, "operator", "init")

	for i := 0; i < 3; i++ {
		vault.Run(t, "operator", "unseal", initOut.UnsealKeys[i])
	}

	// log in with root token
	vault.Run(t, "login", initOut.RootToken)

	return Cmd{vault}
}

func (cmd Cmd) Write(t *testing.T, path string, val interface{}) {
	cmd.WithArgs("write", path).Run(t, writeArgs(val)...)
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
