package drain_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestDrain(t *testing.T) {
	SetDefaultEventuallyTimeout(30 * time.Second)

	RegisterFailHandler(Fail)
	RunSpecs(t, "Drain Suite")
}
