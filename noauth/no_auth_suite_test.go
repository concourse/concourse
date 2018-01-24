package noauth_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestNoAuth(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "No Auth Suite")
}
