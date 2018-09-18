package lockrunner_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLockrunner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lockrunner Suite")
}
