package main_test

import (
	"os/exec"

	"github.com/concourse/concourse/v8"
	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Version flag", func() {
	DescribeTable("prints the version and exits",
		func(args ...string) {
			session, err := gexec.Start(exec.Command(concoursePath, args...), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(Equal(concourse.Version + "\n"))
		},
		Entry("--version", "--version"),
		Entry("web --version", "web", "--version"),
		Entry("worker --version", "worker", "--version"),
		Entry("worker -v", "worker", "-v"),
		Entry("quickstart --version", "quickstart", "--version"),
	)
})
