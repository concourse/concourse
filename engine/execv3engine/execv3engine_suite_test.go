package execv3engine_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestExecv3engine(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Execv3engine Suite")
}
