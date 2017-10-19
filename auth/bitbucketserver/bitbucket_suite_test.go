package bitbucketserver_test

import (
	"testing"
	. "github.com/onsi/gomega"
	. "github.com/onsi/ginkgo"
)

func TestBitbucketServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bitbucket Server Suite")
}
