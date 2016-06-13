package index_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestIndex(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Index Suite")
}
