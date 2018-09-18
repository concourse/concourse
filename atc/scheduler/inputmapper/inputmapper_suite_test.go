package inputmapper_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestInputmapper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Inputmapper Suite")
}
