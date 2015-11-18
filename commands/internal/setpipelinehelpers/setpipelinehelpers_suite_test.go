package setpipelinehelpers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSetpipelinehelpers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Set-Pipeline Helpers Suite")
}
