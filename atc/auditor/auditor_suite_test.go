package auditor_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestAuditor(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Auditor Suite")
}
