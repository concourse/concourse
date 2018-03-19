package integration_test

import (
	"encoding/pem"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/fly/rc"
)

var _ = Describe("login -k Command", func() {
	var loginATCServer *ghttp.Server

	Describe("login", func() {
		var (
			flyCmd *exec.Cmd
			stdin  io.WriteCloser
		)
		BeforeEach(func() {
			l := log.New(GinkgoWriter, "TLSServer", 0)
			loginATCServer = ghttp.NewUnstartedServer()
			loginATCServer.HTTPTestServer.Config.ErrorLog = l
			loginATCServer.HTTPTestServer.StartTLS()
		})

		AfterEach(func() {
			loginATCServer.Close()
		})

		Context("to new target with invalid SSL with -k", func() {
			BeforeEach(func() {
				loginATCServer.AppendHandlers(
					infoHandler(),
					tokenHandler(),
				)

				flyCmd = exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL(), "-k", "-u", "some_user", "-p", "some_pass")

				var err error
				stdin, err = flyCmd.StdinPipe()
				Expect(err).NotTo(HaveOccurred())
			})

			It("succeeds", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Consistently(sess.Out.Contents).ShouldNot(ContainSubstring("some_pass"))

				Eventually(sess.Out).Should(gbytes.Say("target saved"))

				err = stdin.Close()
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
			})

			Context("login to existing target", func() {
				var otherCmd *exec.Cmd
				BeforeEach(func() {
					loginATCServer.AppendHandlers(
						infoHandler(),
						tokenHandler(),
					)

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("target saved"))

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))
				})

				Context("with -k", func() {
					BeforeEach(func() {
						otherCmd = exec.Command(flyPath, "-t", "some-target", "login", "-k", "-u", "some_user", "-p", "some_pass")
					})

					It("succeeds", func() {
						sess, err := gexec.Start(otherCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess.Out).Should(gbytes.Say("target saved"))

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
					})
				})

				Context("without -k", func() {
					BeforeEach(func() {
						otherCmd = exec.Command(flyPath, "-t", "some-target", "login", "-u", "some_user", "-p", "some_pass")
					})

					It("errors", func() {
						sess, err := gexec.Start(otherCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(1))
						Eventually(sess.Err).Should(gbytes.Say("x509: certificate signed by unknown authority"))
					})
				})
			})
		})

		Context("to new target with invalid SSL without -k", func() {
			Context("without --ca-cert", func() {
				BeforeEach(func() {
					flyCmd = exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL(), "-u", "some_user", "-p", "some_pass")

					var err error
					stdin, err = flyCmd.StdinPipe()
					Expect(err).NotTo(HaveOccurred())
				})

				It("errors", func() {
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					err = stdin.Close()
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))
					Eventually(sess.Err).Should(gbytes.Say("x509: certificate signed by unknown authority"))
				})
			})

			Context("with --ca-cert", func() {
				var (
					tmpDir  string
					sslCert string
				)

				BeforeEach(func() {
					sslCert = string(pem.EncodeToMemory(&pem.Block{
						Type:  "CERTIFICATE",
						Bytes: loginATCServer.HTTPTestServer.TLS.Certificates[0].Certificate[0],
					}))

					caCertFile, err := ioutil.TempFile("", "ca_cert.pem")
					Expect(err).NotTo(HaveOccurred())

					_, err = caCertFile.WriteString(sslCert)
					Expect(err).NotTo(HaveOccurred())

					flyCmd = exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL(), "--ca-cert", caCertFile.Name(), "-u", "some_user", "-p", "some_pass")

					loginATCServer.AppendHandlers(
						infoHandler(),
						tokenHandler(),
					)

					tmpDir, err = ioutil.TempDir("", "fly-test")
					Expect(err).NotTo(HaveOccurred())

					os.Setenv("HOME", tmpDir)
				})

				It("succeeds", func() {
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Consistently(sess.Out.Contents).ShouldNot(ContainSubstring("some_pass"))

					Eventually(sess.Out).Should(gbytes.Say("target saved"))

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))

					By("saving the CA cert to the .flyrc", func() {
						returnedTarget, err := rc.LoadTarget("some-target", false)
						Expect(err).NotTo(HaveOccurred())
						Expect(returnedTarget.CACert()).To(Equal(sslCert))
					})
				})
			})
		})

		Context("to existing target with invalid SSL certificate", func() {
			Context("when 'insecure' is not set", func() {
				BeforeEach(func() {
					flyrcContents := `targets:
  some-target:
    api: ` + loginATCServer.URL() + `
    team: main
    ca_cert: some-ca-cert
    token:
      type: Bearer
      value: some-token`
					ioutil.WriteFile(homeDir+"/.flyrc", []byte(flyrcContents), 0777)
				})

				Context("with -k", func() {
					BeforeEach(func() {
						loginATCServer.AppendHandlers(
							infoHandler(),
							tokenHandler(),
						)

						flyCmd = exec.Command(flyPath, "-t", "some-target", "login", "-k", "-u", "some_user", "-p", "some_pass")
					})

					It("succeeds", func() {
						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Consistently(sess.Out.Contents).ShouldNot(ContainSubstring("some_pass"))

						Eventually(sess.Out).Should(gbytes.Say("target saved"))

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))

						By("saving the CA cert to the .flyrc", func() {
							returnedTarget, err := rc.LoadTarget("some-target", false)
							Expect(err).NotTo(HaveOccurred())
							Expect(returnedTarget.CACert()).To(Equal(""))
						})
					})
				})

				Context("without -k", func() {
					BeforeEach(func() {
						flyCmd = exec.Command(flyPath, "-t", "some-target", "login")
					})

					It("errors", func() {
						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(1))
						Eventually(sess.Err).Should(gbytes.Say("CA Cert not valid"))
					})
				})
			})
		})
	})
})
