package integration_test

import (
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Fly CLI", func() {
	Describe("curl", func() {
		var (
			flyCmd *exec.Cmd
		)

		Context("when providing query params with path", func() {
			It("parse the query params correctly", func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "curl", "--print-and-exit", "some-path?some-query-param=value")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))

				Expect(string(sess.Out.Contents())).To(ContainSubstring("some-path?some-query-param=value"))
			})
		})

		Context("when providing curl command flags", func() {
			It("append flags to curl command correctly", func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "curl", "--print-and-exit", "some-path", "--", "-X", "PUT")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))

				Expect(string(sess.Out.Contents())).To(ContainSubstring("-X PUT"))
			})
		})
	})
})
