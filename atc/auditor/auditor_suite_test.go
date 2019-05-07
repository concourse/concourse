package auditor_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAuditor(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Auditor Suite")
}
