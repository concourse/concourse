package tls_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTls(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tls Suite")
}
