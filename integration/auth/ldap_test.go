package auth_test

import (
	"testing"

	"github.com/concourse/concourse/integration/internal/dctest"
	"github.com/concourse/concourse/integration/internal/flytest"
	"github.com/stretchr/testify/require"
)

func TestLDAP_PasswordConnector(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "../docker-compose.yml", "overrides/ldap.yml")
	dc.Run(t, "up", "-d")

	fly, webURL := flytest.InitUnauthenticated(t, dc)

	t.Run("valid credentials", func(t *testing.T) {
		fly.Run(t, "login", "-c", webURL, "-u", "user1@example.com", "-p", "user1pass")
	})

	t.Run("invalid credentials", func(t *testing.T) {
		err := fly.Try("login", "-c", webURL, "-u", "user1@example.com", "-p", "invalid")
		require.Error(t, err)
	})
}
