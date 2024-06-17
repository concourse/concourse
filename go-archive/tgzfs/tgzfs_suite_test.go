package tgzfs_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTgzfs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tgzfs Suite")
}
