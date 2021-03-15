package web_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestWebHandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Web Handler Suite")
}
