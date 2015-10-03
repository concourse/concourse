package paths_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPaths(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Paths Suite")
}
