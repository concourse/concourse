package present_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPresent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Present Suite")
}
