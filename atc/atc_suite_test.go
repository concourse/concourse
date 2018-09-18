package atc_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestATC(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ATC Suite")
}
