package courier_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCourier(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Courier Suite")
}
