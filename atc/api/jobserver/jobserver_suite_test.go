package jobserver_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPipelineserver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Jobserver Suite")
}
