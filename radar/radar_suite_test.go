package radar_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRadar(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Radar Suite")
}
