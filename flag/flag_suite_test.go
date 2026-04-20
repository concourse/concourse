package flag_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestFlag(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Flag Suite")
}
