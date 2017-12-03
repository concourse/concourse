package cloud

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
)

func TestBitbucketCloud(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bitbucket Cloud Suite")
}
