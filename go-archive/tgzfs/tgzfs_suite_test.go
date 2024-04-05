package tgzfs_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTgzfs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tgzfs Suite")
}
