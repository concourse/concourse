package baggageclaimcmd_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBaggageclaimcmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Baggageclaimcmd Suite")
}
