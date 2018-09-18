package inputconfig_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestInputconfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Inputconfig Suite")
}
