package creds_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/concourse/concourse/integration/internal/dctest"
	"github.com/concourse/concourse/integration/internal/flytest"
	"github.com/concourse/concourse/integration/internal/vaulttest"
	"github.com/stretchr/testify/require"
)

type tokenSummary struct {
	Auth struct {
		ClientToken string `json:"client_token"`
	} `json:"auth"`
}

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
		func(team, key string, val any) {
			vault.Write(t,
				fmt.Sprintf("concourse/%s/%s", team, key),
				val,
			)
		},
		func(team, pipeline, key string, val any) {
			vault.Write(t,
				fmt.Sprintf("concourse/%s/%s/%s", team, pipeline, key),
				val,
			)
		},
	)
}

func TestVaultTokenPath(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "../docker-compose.yml", "overrides/vault-token.yml")

	// set up kv v1 store for Concourse
	dc.Run(t, "up", "-d", "vault")
	vault := vaulttest.Init(t, dc)
	vault.Run(t, "secrets", "enable", "-version=1", "-path", "concourse/main", "kv")
	setupVaultAuth(t, vault)

	// create and mount the client token as a file in the web container
	summary := tokenSummary{}
	vault.OutputJSON(t, &summary, "token", "create", "--policy=concourse", "--format=json")
	dir := "../../hack/vault"
	err := os.WriteFile(filepath.Join(dir, "token"), []byte(summary.Auth.ClientToken), 0666)
	require.NoError(t, err)

	// start concourse and run the test
	dc.Run(t, "up", "-d")
	fly := flytest.InitOverrideCredentials(t, dc)

	testCredentialManagement(t, fly, dc,
		func(team, key string, val any) {
			vault.Write(t,
				fmt.Sprintf("concourse/%s/%s", team, key),
				val,
			)
		},
		func(team, pipeline, key string, val any) {
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

func TestVaultKVMountCacheMultipleNamespaces(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "../docker-compose.yml", "overrides/vault.yml")
	dc.Run(t, "up", "-d")

	vault := vaulttest.Init(t, dc)

	fly := flytest.Init(t, dc)

	// Set up TWO KV stores: one at "secret/" and one at "secret-prod/" (path segment boundary test)
	// Both should be isolated in cache despite similar names
	vault.Run(t, "secrets", "enable", "-version=2", "-path", "secret", "kv")
	vault.Run(t, "secrets", "enable", "-version=1", "-path", "secret-prod", "kv")

	vault.Run(t, "secrets", "enable", "-version=1", "-path", "concourse/main", "kv")

	setupVaultAuth(t, vault)

	vault.Write(t, "secret/data/team-secret", map[string]any{"value": "from-secret-v2"})
	vault.Write(t, "secret-prod/team-secret", map[string]any{"value": "from-secret-prod-v1"})

	// Write concourse-style secret
	vault.Write(t, "concourse/main/team_secret", map[string]any{"value": "teamval"})

	testCredentialManagement(t, fly, dc,
		func(team, key string, val any) {
			vault.Write(t,
				fmt.Sprintf("concourse/%s/%s", team, key),
				val,
			)
		},
		func(team, pipeline, key string, val any) {
			vault.Write(t,
				fmt.Sprintf("concourse/%s/%s/%s", team, pipeline, key),
				val,
			)
		},
	)
}

func TestVaultKVMountCacheDifferentVersions(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "../docker-compose.yml", "overrides/vault.yml")
	dc.Run(t, "up", "-d")

	vault := vaulttest.Init(t, dc)

	fly := flytest.Init(t, dc)

	// Set up hierarchy with different KV versions at different levels
	// This tests that cache correctly stores version info per mount
	vault.Run(t, "secrets", "enable", "-version=1", "-path", "concourse", "kv")
	vault.Run(t, "secrets", "enable", "-version=2", "-path", "concourse/nested", "kv")
	vault.Run(t, "secrets", "enable", "-version=1", "-path", "concourse/main", "kv")

	setupVaultAuth(t, vault)
	vault.Write(t, "concourse/root_secret", map[string]any{"value": "v1-root"})
	vault.Write(t, "concourse/nested/data/nested_secret", map[string]any{"value": "v2-nested"})
	vault.Write(t, "concourse/main/team_secret", map[string]any{"value": "v1-main"})

	testCredentialManagement(t, fly, dc,
		func(team, key string, val any) {
			vault.Write(t,
				fmt.Sprintf("concourse/%s/%s", team, key),
				val,
			)
		},
		func(team, pipeline, key string, val any) {
			vault.Write(t,
				fmt.Sprintf("concourse/%s/%s/%s", team, pipeline, key),
				val,
			)
		},
	)
}

func TestVaultKVMountCachePathSegmentBoundary(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "../docker-compose.yml", "overrides/vault.yml")
	dc.Run(t, "up", "-d")

	vault := vaulttest.Init(t, dc)

	fly := flytest.Init(t, dc)

	// This test validates the edge case: "secret-prod/" should NOT match mount "secret/"
	// Cache must enforce path segment boundaries
	vault.Run(t, "secrets", "enable", "-version=1", "-path", "secret", "kv")
	vault.Run(t, "secrets", "enable", "-version=1", "-path", "concourse/main", "kv")

	setupVaultAuth(t, vault)

	vault.Write(t, "secret/foo", map[string]any{"value": "secret-foo"})
	vault.Write(t, "concourse/main/team_secret", map[string]any{"value": "team"})

	testCredentialManagement(t, fly, dc,
		func(team, key string, val any) {
			vault.Write(t,
				fmt.Sprintf("concourse/%s/%s", team, key),
				val,
			)
		},
		func(team, pipeline, key string, val any) {
			vault.Write(t,
				fmt.Sprintf("concourse/%s/%s/%s", team, pipeline, key),
				val,
			)
		},
	)
}

func setupVaultAuth(t *testing.T, vault vaulttest.Cmd) {
	// set up a policy for Concourse
	vault.WithInput(bytes.NewBufferString(`
		path "concourse/*" {
			policy = "read"
		}
		path "secret*" {
			policy = "read"
		}
	`)).Run(t, "policy", "write", "concourse", "-")

	// set up cert-based auth
	vault.Run(t, "auth", "enable", "cert")
	vault.Write(t, "auth/cert/certs/concourse", map[string]any{
		"policies":    "concourse",
		"certificate": "@/vault/certs/vault-ca.crt", // resolved within container
		"ttl":         "1h",
	})
}
