package version_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestVersion(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Version Suite")
}
