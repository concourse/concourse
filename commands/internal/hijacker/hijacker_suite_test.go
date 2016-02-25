package hijacker_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestHijacker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hijacker Suite")
}
