package skycmd_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
)

func TestDexServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SkyCmd Suite")
}

var _ = BeforeSuite(func() {
})

var _ = AfterSuite(func() {
})
