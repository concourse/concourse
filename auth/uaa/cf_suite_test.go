package uaa_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestUaa(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Uaa Suite")
}
