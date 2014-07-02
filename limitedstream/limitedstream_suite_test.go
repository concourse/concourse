package limitedstream_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLimitedstream(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Limitedstream Suite")
}
