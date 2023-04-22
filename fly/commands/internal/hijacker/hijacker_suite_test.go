package hijacker_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestHijacker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hijacker Suite")
}
