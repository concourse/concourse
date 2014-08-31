package logfanout_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLogfanout(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Logfanout Suite")
}
