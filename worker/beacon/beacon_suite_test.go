package beacon_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestBeacon(t *testing.T) {
	SetDefaultEventuallyTimeout(time.Minute)

	RegisterFailHandler(Fail)
	RunSpecs(t, "Beacon Suite")
}
