package ops_test

import (
	"fmt"
	"strings"

	"github.com/concourse/concourse/integration/cmdtest"
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

func (s *OpsSuite) testCredentialManagement(
	fly cmdtest.Cmd,
	dc cmdtest.Cmd,
	setTeamVar func(string, string, interface{}),
	setPipelineVar func(string, string, string, interface{}),
) {
	for name, val := range teamVars {
		setTeamVar("main", name, val)
	}

	for name, val := range pipelineVars {
		setPipelineVar("main", "test", name, val)
	}

	s.Run("pipelines", func() {
		err := fly.Run("set-pipeline", "-n", "-c", "pipelines/credential-management.yml", "-p", "test")
		s.NoError(err)

		err = fly.Run("unpause-pipeline", "-p", "test")
		s.NoError(err)

		s.Run("config is not interpolated", func() {
			config, err := fly.Output("get-pipeline", "-p", "test")
			s.NoError(err)

			eachString(pipelineVars, func(val string) {
				if val != "" {
					s.NotContains(config, val)
				}
			})

			eachString(teamVars, func(val string) {
				if val != "" {
					s.NotContains(config, val)
				}
			})
		})

		s.Run("interpolates resource type checks", func() {
			// build will fail if ((check_failure)) doesn't get interpolated to ""
			err := fly.Run("check-resource-type", "-r", "test/custom-resource-type")
			s.NoError(err)
		})

		s.Run("interpolates resource checks", func() {
			// build will fail if ((check_failure)) doesn't get interpolated to ""
			err = fly.Run("check-resource", "-r", "test/custom-resource")
			s.NoError(err)
		})

		s.Run("interpolates job builds", func() {
			// build will fail and return err if any values are wrong
			err := fly.Run("trigger-job", "-w", "-j", "test/some-job")
			s.NoError(err)
		})

		s.Run("interpolates one-off builds with job inputs", func() {
			// build will fail and return err if any values are wrong
			err := fly.WithEnv(
				"EXPECTED_RESOURCE_SECRET=some resource secret",
				"EXPECTED_RESOURCE_VERSION_SECRET=exposed some version not-so-secret",
			).Run(
				"execute",
				"-c", "tasks/credential-management-with-job-inputs.yml",
				"-j", "test/some-job",
			)
			s.NoError(err)
		})
	})

	s.Run("interpolates one-off builds", func() {
		// build will fail and return err if any values are wrong
		err := fly.WithEnv(
			"EXPECTED_TEAM_SECRET=some team secret",
			"EXPECTED_RESOURCE_VERSION_SECRET=exposed some version not-so-secret",
		).Run("execute", "-c", "tasks/credential-management.yml")
		s.NoError(err)
	})

	s.Run("does not store secrets in database", func() {
		pgDump := dc.WithArgs("exec", "-T", "db", "pg_dump")

		dump, err := pgDump.Silence().Output(
			"--exclude-schema=build_id_seq_*",
			"-U", "dev",
			"concourse",
		)
		s.NoError(err)

		eachString(pipelineVars, func(val string) {
			if val != "" && !strings.HasPrefix(val, "exposed ") {
				s.NotContains(dump, val)
			}
		})

		eachString(teamVars, func(val string) {
			if val != "" && !strings.HasPrefix(val, "exposed ") {
				s.NotContains(dump, val)
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
