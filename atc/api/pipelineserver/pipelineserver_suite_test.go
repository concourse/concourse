package pipelineserver_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPipelineserver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pipelineserver Suite")
}
