package hijackhelpers_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestHijackhelpers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hijackhelpers Suite")
}
