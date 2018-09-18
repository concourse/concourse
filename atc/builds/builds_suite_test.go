package builds_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestBuilds(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Builds Suite")
}
