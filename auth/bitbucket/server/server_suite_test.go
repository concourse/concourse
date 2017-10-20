package server

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
)

func TestBitbucketServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bitbucket Server Suite")
}
