package ansistream_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAnsistream(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ansistream Suite")
}
