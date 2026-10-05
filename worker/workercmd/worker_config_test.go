package workercmd_test

import (
	"testing"

	"github.com/concourse/concourse/v8/worker/workercmd"
)

func TestWorkerConfig_P2PStreamingGroup(t *testing.T) {
	config := workercmd.WorkerConfig{
		Name:              "test-worker",
		P2PStreamingGroup: "group-a",
	}

	w := config.Worker()
	if w.P2PStreamingGroup != "group-a" {
		t.Errorf("expected P2PStreamingGroup to be %q, got %q", "group-a", w.P2PStreamingGroup)
	}
}

func TestWorkerConfig_P2PStreamingGroupEmpty(t *testing.T) {
	config := workercmd.WorkerConfig{
		Name: "test-worker",
	}

	w := config.Worker()
	if w.P2PStreamingGroup != "" {
		t.Errorf("expected P2PStreamingGroup to be empty, got %q", w.P2PStreamingGroup)
	}
}
