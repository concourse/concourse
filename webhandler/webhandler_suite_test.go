package webhandler_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestWebhandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Web Handler Suite")
}
