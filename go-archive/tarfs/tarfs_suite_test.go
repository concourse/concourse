package tarfs_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTarfs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tarfs Suite")
}
