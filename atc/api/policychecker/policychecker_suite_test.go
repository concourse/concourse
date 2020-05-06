package policychecker_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAccessor(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API PolicyChecker Suite")
}
