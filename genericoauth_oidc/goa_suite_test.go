package genericoauth_oidc_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGoaOIDC(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Generic OAuth OIDC Suite")
}
