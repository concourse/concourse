package auditor_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAuditor(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Auditor Suite")
}
