package dbgc_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestDbgc(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dbgc Suite")
}
