package bitbucketcloud_test

import (
	"testing"
	. "github.com/onsi/gomega"
	. "github.com/onsi/ginkgo"
)

func TestBitbucket(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bitbucket Cloud Suite")
}
