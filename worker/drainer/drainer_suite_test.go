package drainer_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestDrainer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Drainer Suite")
}
