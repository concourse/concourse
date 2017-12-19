package ssm_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSsm(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ssm Creds Suite")
}
