package conjur_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestConjur(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Conjur Suite")
}
