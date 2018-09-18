package integration_test

import (
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Fly CLI", func() {
	Describe("auth failures", func() {
		var (
			flyCmd *exec.Cmd
		)

		BeforeEach(func() {
			flyCmd = exec.Command(flyPath, "-t", targetName, "containers")
		})

		Context("when a 401 response is received", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/containers"),
						ghttp.RespondWith(401, ""),
					),
				)
			})

			It("instructs the user to log in", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say("not authorized\\. run the following to log in:\n\n    "))
				Expect(sess.Err).To(gbytes.Say(`fly -t ` + targetName + ` login`))
			})
		})
	})

	Describe("missing target", func() {
		var (
			flyCmd *exec.Cmd
		)

		BeforeEach(func() {
			flyCmd = exec.Command(flyPath, "containers")
		})

		It("instructs the user to specify a target", func() {
			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(1))

			Expect(sess.Err).To(gbytes.Say("no target specified\\. specify the target with -t or log in like so:"))
			Expect(sess.Err).To(gbytes.Say(`fly -t \(alias\) login -c \(concourse url\)`))
		})
	})

	Describe("network errors", func() {
		var (
			flyCmd *exec.Cmd
		)

		BeforeEach(func() {
			atcServer.Close()

			flyCmd = exec.Command(flyPath, "-t", targetName, "containers")
		})

		It("tells the user a network error occurred, and that their target may be wrong, and makes fun of them", func() {
			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(1))

			Expect(sess.Err).To(gbytes.Say("could not reach the Concourse server called " + targetName))
			Expect(sess.Err).To(gbytes.Say("lol"))
		})
	})
})
