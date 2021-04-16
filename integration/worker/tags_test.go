package main

import (
	"testing"

	"github.com/concourse/concourse/integration/internal/dctest"
	"github.com/concourse/concourse/integration/internal/flytest"
)

func TestTags_EmptyString(t *testing.T) {
	t.Parallel()

	dc := dctest.Init(t, "../docker-compose.yml", "overrides/empty_tag.yml")
	dc.Run(t, "up", "-d")

	fly := flytest.Init(t, dc)
	t.Run("run untagged task on a worker with an empty tag", func(t *testing.T) {
		fly.Run(t, "execute", "-c", "../tasks/hello.yml")
	})

	t.Run("run task with empty tag on a worker with an empty tag", func(t *testing.T) {
		fly.Run(t, "execute", "-c", "../tasks/hello.yml", "--tag", "")
	})
}
