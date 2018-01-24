package basicauth_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestBasicAuth(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Basic Auth Suite")
}
