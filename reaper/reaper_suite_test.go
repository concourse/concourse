package reaper_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestReaper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reaper Suite")
}
