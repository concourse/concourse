package algorithm_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAlgorithm(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Algorithm Suite")
}
