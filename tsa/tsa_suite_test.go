package tsa_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTSA(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TSA Suite")
}
