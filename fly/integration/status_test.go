package integration_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("status Command", func() {
	var (
		tmpDir string
		flyrc  string
		flyCmd *exec.Cmd
	)
	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "fly-test")
		Expect(err).ToNot(HaveOccurred())

		os.Setenv("HOME", tmpDir)
		flyrc = filepath.Join(userHomeDir(), ".flyrc")

		flyFixtureData := []byte(`
targets:
  another-test:
    api: ` + atcServer.URL() + `
    team: test
    token:
      type: Bearer
      value: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOiIxNDU4MzUyNDcwIiwiaXNBZG1pbiI6Im5vcGUiLCJ0ZWFtSUQiOjEsInRlYW1OYW1lIjoibWFpbiJ9.v04hbwIFdMNjp6BCpz2jvOYNpAeBY8pio6hlXQizLAM
  bad-test:
    api: https://example.com/another-test
    team: test
    token:
      type: Bearer
      value: bad-token
  loggedout-test:
    api: https://example.com/loggedout-test
    team: test
    token:
      type: ""
      value: ""
  invalid-test:
    api: https://example.com/invalid-test
    team: test
    token:
  expired-test:
    api: https://example.com/expired-test
    team: test
    token:
      type: Bearer
      value: eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJjc3JmIjoiNGZlOGM1Y2VhYjFjNWFiMzE5MzMzNmQ2YThkOTE3OTJmZTA0YmEzZjg5Y2IwZDg0MTkzN2I2MzkzYTdhYTg2MyIsImV4cCI6MTUyMTMwODk2MCwiaXNBZG1pbiI6dHJ1ZSwidGVhbU5hbWUiOiJtYWluIn0.oyb-2CPLnXy-7S-9FWWlx106KI5Xpd6B5XIrFOvcG1yyh5nrGpM4NfgaW7ugN4zzi2mSFGawRlkulzgAZ4RxAEdTOnlSXvVZO3vD70sMlrp_LX-lYaqJ7XXVXNKvKE_74YGZY414TYVy2IxL-4Qf7pbb0uGDky03jQFxkWVSUiD5iLwaqpvxpHTEuVNoZc9a8YNiOdETvqnt50drsmxpkblM60DrWuDVPifOfTrooSMxULnl3pYXDsTPZbrc6QVLA_Hpi7wWCNEZbAojTQ3taIwzp7BBAuxUNcVMpJKy3Um5oMHcibe1R0PsZ0J49PbLSclZfhJ7wjHBc7FQEKTZzQ
`)

		err = ioutil.WriteFile(flyrc, flyFixtureData, 0600)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Context("status with no target name", func() {
		var (
			flyCmd *exec.Cmd
		)
		BeforeEach(func() {
			flyCmd = exec.Command(flyPath, "status")
		})

		It("instructs the user to specify --target", func() {
			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(1))

			Expect(sess.Err).To(gbytes.Say(`no target specified. specify the target with -t`))
		})
	})

	Context("status with target name", func() {
		Context("when target is saved with valid token", func() {
			BeforeEach(func() {
				atcServer.Reset()
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/user"),
						ghttp.RespondWithJSONEncoded(200, map[string]interface{}{"team": "test"}),
					),
				)
			})

			It("command exist with 0", func() {
				flyCmd = exec.Command(flyPath, "-t", "another-test", "status")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))

				Expect(sess.Out).To(gbytes.Say(`logged in successfully`))
			})
		})

		Context("when target is saved with valid token but is unauthorized on server", func() {
			BeforeEach(func() {
				atcServer.Reset()
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/user"),
						ghttp.RespondWith(401, nil),
					),
				)
			})

			It("command exist with 1", func() {
				flyCmd = exec.Command(flyPath, "-t", "another-test", "status")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say(`please login again`))
				Expect(sess.Err).To(gbytes.Say(`token validation failed with error : not authorized`))
			})
		})

		Context("when target is saved with invalid token", func() {
			It("command exist with 1", func() {
				flyCmd = exec.Command(flyPath, "-t", "bad-test", "status")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say(`please login again`))
				Expect(sess.Err).To(gbytes.Say(`token validation failed with error : token contains an invalid number of segments`))
			})
		})

		Context("when target is saved with expired token", func() {
			It("command exist with 1", func() {
				flyCmd = exec.Command(flyPath, "-t", "expired-test", "status")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say(`please login again`))
				Expect(sess.Err).To(gbytes.Say(`token validation failed with error : Token is expired`))
			})
		})

		Context("when target is logged out", func() {
			It("command exist with 1 and log out msg", func() {
				flyCmd = exec.Command(flyPath, "-t", "loggedout-test", "status")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
				Expect(sess.Err).To(gbytes.Say(`logged out`))
			})
		})

		Context("when invalid token is in target", func() {
			It("command exist with 1", func() {
				flyCmd = exec.Command(flyPath, "-t", "invalid-test", "status")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
				Expect(sess.Err).To(gbytes.Say(`logged out`))
			})
		})

		Context("when unknown target is used", func() {
			It("command exist with 1", func() {
				flyCmd = exec.Command(flyPath, "-t", "unknown-test", "status")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say(`unknown target: unknown-test`))
			})
		})
	})
})
