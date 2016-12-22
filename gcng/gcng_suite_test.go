package gcng_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGcng(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gcng Suite")
}
