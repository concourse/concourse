package creds_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/concourse/concourse/integration/internal/dctest"
	"github.com/concourse/concourse/integration/internal/flytest"
	"github.com/stretchr/testify/require"
)

const assertionScript = `#!/bin/sh

# add -x to troubleshoot
#
# remember to remove it after so it doesn't get saved to build logs
set -e

test "$SECRET_USERNAME" = "some username"
test "$SECRET_PASSWORD" = "some password"
test "$TEAM_SECRET" = "some team secret"

test "$MIRRORED_VERSION" = "exposed some version not-so-secret"

test "$(cat some-resource/resource_secret)" = "some resource secret"
test "$(cat custom-resource/custom_resource_secret)" = "some resource secret"
test "$(cat params-in-get/username)" = "get some username"
test "$(cat params-in-get/password)" = "get some password"
test "$(cat params-in-put/version)" = "exposed some version not-so-secret"
test "$(cat params-in-put/username)" = "put-get some username"
test "$(cat params-in-put/password)" = "put-get some password"

echo all credentials matched expected values
`

var pipelineVars = map[string]interface{}{
	"assertion_script":     assertionScript,
	"check_failure":        "", // blank so the check doesn't fail
	"resource_type_secret": "some resource type secret",
	"resource_secret":      "some resource secret",
	"job_secret": map[string]interface{}{
		"username": "some username",
		"password": "some password",
	},
}

var teamVars = map[string]interface{}{
	"team_secret":      "some team secret",
	"resource_version": "exposed some version not-so-secret",
}

func testCredentialManagement(
	t *testing.T,
	fly flytest.Cmd,
	dc dctest.Cmd,
	setTeamVar func(string, string, interface{}),
	setPipelineVar func(string, string, string, interface{}),
) {
	for name, val := range teamVars {
		setTeamVar("main", name, val)
	}

	for name, val := range pipelineVars {
		setPipelineVar("main", "test", name, val)
	}

	t.Run("pipelines", func(t *testing.T) {
		fly.Run(t, "set-pipeline", "-n", "-c", "pipelines/credential-management.yml", "-p", "test")
		fly.Run(t, "unpause-pipeline", "-p", "test")

		t.Run("config is not interpolated", func(t *testing.T) {
			config := fly.Output(t, "get-pipeline", "-p", "test")

			eachString(pipelineVars, func(val string) {
				if val != "" {
					require.NotContains(t, config, val)
				}
			})

			eachString(teamVars, func(val string) {
				if val != "" {
					require.NotContains(t, config, val)
				}
			})
		})

		t.Run("interpolates resource type checks", func(t *testing.T) {
			// build will fail if ((check_failure)) doesn't get interpolated to ""
			fly.Run(t, "check-resource-type", "-r", "test/custom-resource-type")
		})

		t.Run("interpolates resource checks", func(t *testing.T) {
			// build will fail if ((check_failure)) doesn't get interpolated to ""
			fly.Run(t, "check-resource", "-r", "test/custom-resource")
		})

		t.Run("interpolates job builds", func(t *testing.T) {
			// build will fail and return err if any values are wrong
			fly.Run(t, "trigger-job", "-w", "-j", "test/some-job")
		})

		t.Run("interpolates one-off builds with job inputs", func(t *testing.T) {
			// build will fail and return err if any values are wrong
			fly.WithEnv(
				"EXPECTED_RESOURCE_SECRET=some resource secret",
				"EXPECTED_RESOURCE_VERSION_SECRET=exposed some version not-so-secret",
			).Run(t,
				"execute",
				"-c", "tasks/credential-management-with-job-inputs.yml",
				"-j", "test/some-job",
			)
		})
	})

	t.Run("interpolates one-off builds", func(t *testing.T) {
		// build will fail and return err if any values are wrong
		fly.WithEnv(
			"EXPECTED_TEAM_SECRET=some team secret",
			"EXPECTED_RESOURCE_VERSION_SECRET=exposed some version not-so-secret",
		).Run(t, "execute", "-c", "tasks/credential-management.yml")
	})

	t.Run("does not store secrets in database", func(t *testing.T) {
		pgDump := dc.WithArgs("exec", "-T", "db", "pg_dump")

		dump := pgDump.Silence().Output(t,
			"--exclude-schema=build_id_seq_*",
			"-U", "dev",
			"concourse",
		)

		eachString(pipelineVars, func(val string) {
			if val != "" && !strings.HasPrefix(val, "exposed ") {
				require.NotContains(t, dump, val)
			}
		})

		eachString(teamVars, func(val string) {
			if val != "" && !strings.HasPrefix(val, "exposed ") {
				require.NotContains(t, dump, val)
			}
		})
	})
}

func eachString(val interface{}, f func(string)) {
	switch x := val.(type) {
	case string:
		f(x)
	case map[string]interface{}:
		for _, v := range x {
			eachString(v, f)
		}
	case []interface{}:
		for _, v := range x {
			eachString(v, f)
		}
	default:
		panic(fmt.Sprintf("cannot traverse %T", val))
	}
}
