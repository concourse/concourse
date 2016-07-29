package verifier_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestVerifier(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Verifier Suite")
}
