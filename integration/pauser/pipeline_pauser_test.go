package pauser_test

import (
	"testing"
	"time"

	"github.com/concourse/concourse/integration/internal/dctest"
	"github.com/concourse/concourse/integration/internal/flytest"
	"github.com/stretchr/testify/require"
)

func TestPipelinePauser(t *testing.T) {
	t.Parallel()

	pastDc := dctest.Init(t, "../docker-compose.yml", "overrides/pauser-config.yml", "overrides/five-days-ago.yml")
	pastDc.Run(t, "up", "-d", "--build")

	fly := flytest.Init(t, pastDc)
	t.Run("set and run test pipeline", func(t *testing.T) {
		fly.Run(t, "set-pipeline", "-p", "pauser-test", "-c", "pipelines/one-job.yml", "-n")
		stopUnpausing := make(chan int)
		go func() {
			// sometimes the pipeline pauser pauses the pipeline even after we
			// unpaused it. This workaround persistently unpauses the pipeline
			// until the job is done running
			for {
				select {
				case <-stopUnpausing:
					return
				default:
					fly.Run(t, "unpause-pipeline", "-p", "pauser-test")
					time.Sleep(10 * time.Second)
				}
			}
		}()
		fly.Run(t, "trigger-job", "-j", "pauser-test/one-job", "-w")
		close(stopUnpausing)
	})

	presentDc := dctest.Init(t, "../docker-compose.yml", "overrides/pauser-config.yml", "overrides/short-pauser-interval.yml")
	presentDc.Run(t, "up", "-d", "--build")

	fly = flytest.Init(t, presentDc)
	require.Eventually(t, func() bool {
		return fly.PipelineIsPaused(t, "pauser-test")
	}, 1*time.Minute, 5*time.Second)
}
