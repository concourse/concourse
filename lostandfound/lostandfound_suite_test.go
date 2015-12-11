package lostandfound_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLostandfound(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lost and Found Suite")
}
