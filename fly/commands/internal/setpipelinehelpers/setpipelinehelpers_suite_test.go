package setpipelinehelpers_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSetpipelinehelpers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Set-Pipeline Helpers Suite")
}
