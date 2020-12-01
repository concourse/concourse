package opa_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestOPA(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Policy Agent Suite")
}
