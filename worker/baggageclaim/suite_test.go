package baggageclaim_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestBaggageClaim(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "BaggageClaim Client Suite")
}
