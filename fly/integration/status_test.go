package integration_test

import (
	"io/ioutil"
	"net/http"
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
  with-token:
    api: ` + atcServer.URL() + `
    team: test
    token:
      type: Bearer
      value: some-nice-opaque-access-token
  without-token:
    api: https://example.com/another-test
    team: test
    token:
      type: ""
      value: ""
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
				flyCmd = exec.Command(flyPath, "-t", "with-token", "status")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))

				Expect(sess.Out).To(gbytes.Say(`logged in successfully`))
			})
		})

		Context("when target is saved with a token that is rejected by the server", func() {
			BeforeEach(func() {
				atcServer.Reset()
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/user"),
						ghttp.RespondWith(http.StatusUnauthorized, nil),
					),
				)
			})

			It("the command fails", func() {
				flyCmd = exec.Command(flyPath, "-t", "with-token", "status")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say(`please login again`))
				Expect(sess.Err).To(gbytes.Say(`token validation failed with error: not authorized`))
			})
		})

		Context("when target is saved with invalid token", func() {
			It("the command fails", func() {
				flyCmd = exec.Command(flyPath, "-t", "without-token", "status")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say(`logged out`))
			})
		})
	})
})
