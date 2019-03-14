package templatehelpers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTemplatehelpers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Template Helpers Test Suite")
}
