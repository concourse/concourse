package utf8stream_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestUTF8Stream(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "UTF8Stream Suite")
}
