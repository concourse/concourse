package exec_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"

	. "github.com/concourse/atc/exec"
)

func TestExec(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Exec Suite")
}

func successResult(result Success) func(dest interface{}) bool {
	defer GinkgoRecover()

	return func(dest interface{}) bool {
		defer GinkgoRecover()

		switch x := dest.(type) {
		case *Success:
			*x = result
			return true

		default:
			return false
		}
	}
}

type testMetadata []string

func (m testMetadata) Env() []string { return m }
