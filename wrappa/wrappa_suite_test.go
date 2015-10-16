package wrappa_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestWrappa(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Wrappa Suite")
}
