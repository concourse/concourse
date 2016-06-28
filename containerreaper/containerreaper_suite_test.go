package containerreaper_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestContainerreaper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Containerreaper Suite")
}
