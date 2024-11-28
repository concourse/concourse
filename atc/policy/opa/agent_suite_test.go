package opa_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPolicyAgent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Policy Agent Suite")
}
