package buildreaper_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestBuildreaper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Build Reaper Suite")
}
