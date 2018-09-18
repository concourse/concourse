package integration_test

import (
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Fly CLI", func() {
	Describe("help", func() {
		It("prints help", func() {
			flyCmd := exec.Command(flyPath, "help")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))

			Expect(sess.Out).To(gbytes.Say("Usage:"))
			Expect(sess.Out).To(gbytes.Say("Application Options:"))
			Expect(sess.Out).To(gbytes.Say("Help Options:"))
			Expect(sess.Out).To(gbytes.Say("Available commands:"))
		})
	})

	Context("when invoking binary without flags", func() {
		It("prints help", func() {
			flyCmd := exec.Command(flyPath)

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))

			Expect(sess.Out).To(gbytes.Say("Usage:"))
			Expect(sess.Out).To(gbytes.Say("Application Options:"))
			Expect(sess.Out).To(gbytes.Say("Help Options:"))
			Expect(sess.Out).To(gbytes.Say("Available commands:"))
		})
	})
})
