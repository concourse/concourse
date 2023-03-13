package volume_test

import (
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestVolume(t *testing.T) {
	suiteName := "Volume Suite"
	if runtime.GOOS != "linux" {
		suiteName = suiteName + " - skipping btrfs tests because non-linux"
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, suiteName)
}
