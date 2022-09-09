package validatepipelinehelpers_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestValidatepipelinehelpers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Validate-Pipeline Helpers Suite")
}
