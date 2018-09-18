package beacon_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestBeacon(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Beacon Suite")
}
