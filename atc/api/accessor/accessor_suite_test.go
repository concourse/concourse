package accessor_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAccessor(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Accessor Suite")
}
