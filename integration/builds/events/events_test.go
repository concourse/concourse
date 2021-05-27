package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/concourse/concourse/integration/internal/dctest"
	"github.com/concourse/concourse/integration/internal/flytest"
	"github.com/stretchr/testify/require"
)

func TestLoggingErrorEvents(t *testing.T) {
	dc := dctest.Init(t, "../../docker-compose.yml")

	t.Run("deploy concourse", func(t *testing.T) {
		dc.Run(t, "up", "-d")
	})

	fly := flytest.Init(t, dc)
	fly.Run(t, "set-pipeline", "-n", "-c", "pipelines/erroring_pipeline.yml", "-p", "test")
	fly.Run(t, "unpause-pipeline", "-p", "test")

	cases := map[string]struct {
		JobName                string
		expectedNumberOfErrors int
	}{
		"across step logs errors from sub-steps": {
			JobName:                "across-step",
			expectedNumberOfErrors: 2,
		},
	}

	for description, test := range cases {
		t.Run(description, func(t *testing.T) {
			fly.ExpectExitCode = 2
			fly.Run(t, "tj", "-j", "test/"+test.JobName, "--watch")

			fly.ExpectExitCode = 0
			buildsRaw := fly.Output(t, "curl", "api/v1/teams/main/pipelines/test/jobs/"+test.JobName+"/builds")

			builds := []struct {
				ApiUrl string `json:"api_url"`
			}{}
			json.Unmarshal([]byte(buildsRaw), &builds)
			// Timeout is specified because the event stream never closes
			fly.ExpectExitCode = 1
			buildEvents := fly.Output(t, "curl", builds[0].ApiUrl+"/events", "--", "--max-time", "5")
			errEvents := strings.Count(buildEvents, `"event":"error"`)
			require.Equal(t, errEvents, test.expectedNumberOfErrors, "number of error events is incorrect")
		})
	}
}
