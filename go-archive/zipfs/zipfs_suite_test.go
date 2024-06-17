package zipfs_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestZipfs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Zipfs Suite")
}
