package group_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGroup(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Group Suite")
}
