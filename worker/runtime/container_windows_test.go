//go:build windows

package runtime

import (
	"io"
	"testing"

	"code.cloudfoundry.org/garden"
)

func TestProcessIOCreatorKeepsContainersApart(t *testing.T) {
	stdio := containerdCIO(garden.ProcessIO{Stdout: io.Discard, Stderr: io.Discard}, false)

	first, err := processIOCreator("container-a", stdio)("task")
	if err != nil {
		t.Fatalf("first container: %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })

	second, err := processIOCreator("container-b", stdio)("task")
	if err != nil {
		t.Fatalf("second container running a process with the same ID: %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })
}
