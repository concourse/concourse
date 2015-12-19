package provider_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestProviderFactory(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ProviderFactory Suite")
}
