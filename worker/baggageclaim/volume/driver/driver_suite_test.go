package driver_test

import (
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestDriver(t *testing.T) {
	suiteName := "Driver Suite"
	if runtime.GOOS != "linux" {
		suiteName = suiteName + " - skipping btrfs tests because non-linux"
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, suiteName)
}
