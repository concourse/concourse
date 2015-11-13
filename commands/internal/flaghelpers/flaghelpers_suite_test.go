package flaghelpers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestFlagHelpers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Flag Helpers Suite")
}
