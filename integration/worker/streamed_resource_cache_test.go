package main

import (
	"strings"
	"testing"

	"github.com/concourse/concourse/integration/internal/dctest"
	"github.com/concourse/concourse/integration/internal/flytest"
	"github.com/concourse/concourse/integration/internal/ypath"
	"github.com/stretchr/testify/require"
)

func TestStreamed_ResourceCache(t *testing.T) {
	t.Parallel()

	dockerComposeDoc := ypath.Load(t, "../docker-compose.yml")

	// Configure two worker services in docker-compose with distinct names and tags
	dockerComposeDoc.Move(t, "$.services.worker", "$.services.worker1")
	dockerComposeDoc.Clone(t, "$.services.worker1", "$.services.worker2")

	dockerComposeDoc.Set(t, "$.services.worker1.environment.CONCOURSE_NAME", "worker1")
	dockerComposeDoc.Set(t, "$.services.worker1.environment.CONCOURSE_TAG", "tag1")

	dockerComposeDoc.Set(t, "$.services.worker2.environment.CONCOURSE_NAME", "worker2")
	dockerComposeDoc.Set(t, "$.services.worker2.environment.CONCOURSE_TAG", "tag2")

	dc := dctest.InitDynamic(t, dockerComposeDoc, "..")

	t.Run("deploy with 2 tagged workers", func(t *testing.T) {
		dc.Run(t, "up", "-d")
	})

	fly := flytest.Init(t, dc)
	fly.Run(t, "set-pipeline", "-n", "-c", "pipelines/streamed_resource_cache.yml", "-p", "test")
	fly.Run(t, "unpause-pipeline", "-p", "test")

	// First, trigger a build and the build should run successfully.
	output := fly.Output(t, "trigger-job", "-j", "test/job", "--watch")
	require.Contains(t, output, "hello-world")

	// Then, verify that mock resource has been streamed to worker2 and available on worker2.
	workers := findWorkersContainingMockResource(t, fly)
	require.Equal(t, 2, len(workers))
	require.ElementsMatch(t, workers, []string{"worker1", "worker2"})

	// At last, prune worker1 where the mock resource was generated, then
	// resource on worker2 should be gc-ed as well.
	fly.Run(t, "land-worker", "-w", "worker1")
	fly.Run(t, "prune-worker", "-w", "worker1")
	workers = findWorkersContainingMockResource(t, fly)
	require.Equal(t, 0, len(workers))
}

func findWorkersContainingMockResource(t *testing.T, fly flytest.Cmd) []string {
	table := fly.Table(t, "volumes")
	var workers []string
	for _, row := range table {
		if row["type"] == "resource" && strings.Contains(row["identifier"], "mock") {
			workers = append(workers, row["worker"])
		}
	}
	return workers
}
