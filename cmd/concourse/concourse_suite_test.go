package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

func TestConcourse(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Concourse Suite")
}

var concoursePath string

var _ = SynchronizedBeforeSuite(func() []byte {
	buildPath, err := gexec.Build("github.com/concourse/concourse/cmd/concourse")
	Expect(err).NotTo(HaveOccurred())
	return []byte(buildPath)
}, func(data []byte) {
	concoursePath = string(data)
})

var _ = SynchronizedAfterSuite(func() {
	// other nodes don't need to do any clean up, as it's already taken care of by the first node
}, func() {
	gexec.CleanupBuildArtifacts()
})
