package hijackhelpers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestHijackhelpers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hijackhelpers Suite")
}
