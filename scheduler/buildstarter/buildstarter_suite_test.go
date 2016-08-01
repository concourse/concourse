package buildstarter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestBuildstarter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Buildstarter Suite")
}
