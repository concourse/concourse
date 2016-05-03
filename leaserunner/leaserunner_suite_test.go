package leaserunner_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLeaserunner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lease Runner Suite")
}
