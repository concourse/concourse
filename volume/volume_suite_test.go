package volume_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestVolume(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Volume Suite")
}
