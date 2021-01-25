package ops_test

import (
	"fmt"

	"github.com/concourse/concourse/integration/cmdtest"
)

func (s *OpsSuite) TestVault() {
	dc, err := s.dockerCompose("overrides/vault.yml")
	s.NoError(err)

	s.NoError(dc.Run("up", "-d"))

	vault := s.initVault(dc)

	fly := s.initFly(dc)

	s.testCredentialManagement(fly, dc,
		func(team, key string, val interface{}) {
			path := fmt.Sprintf("concourse/%s/%s", team, key)
			err := vault.WithArgs("write", path).Run(vaultWriteArgs(val)...)
			s.NoError(err)
		},
		func(team, pipeline, key string, val interface{}) {
			path := fmt.Sprintf("concourse/%s/%s/%s", team, pipeline, key)
			err := vault.WithArgs("write", path).Run(vaultWriteArgs(val)...)
			s.NoError(err)
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

func (s *OpsSuite) initVault(dc cmdtest.Cmd) cmdtest.Cmd {
	init := dc.WithArgs("exec", "-T", "vault", "vault")

	var initOut struct {
		UnsealKeys []string `json:"unseal_keys_b64"`
		RootToken  string   `json:"root_token"`
	}

	err := init.OutputJSON(&initOut, "operator", "init")
	s.NoError(err)

	for i := 0; i < 3; i++ {
		err := init.Run("operator", "unseal", initOut.UnsealKeys[i])
		s.NoError(err)
	}

	vault := dc.WithArgs(
		"exec",
		"--env", "VAULT_TOKEN="+initOut.RootToken,
		"-T",    // do not use a TTY
		"vault", // service
		"vault", // command
	)

	// set up kv v1 store for Concourse
	s.NoError(vault.Run("secrets", "enable", "-version=1", "-path", "concourse/main", "kv"))

	// set up a policy for Concourse
	s.NoError(vault.Run("policy", "write", "concourse", "vault/config/concourse-policy.hcl"))

	// set up cert-based auth
	s.NoError(vault.Run("auth", "enable", "cert"))
	s.NoError(vault.Run(
		"write", "auth/cert/certs/concourse",
		"policies=concourse",
		"certificate=@vault/certs/vault-ca.crt",
		"ttl=1h",
	))

	return vault
}
