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

		flyFixtureFile, err := os.OpenFile("./fixtures/status_fly_rc.yml", os.O_RDONLY, 0600)
		Expect(err).NotTo(HaveOccurred())

		flyFixtureData, err := ioutil.ReadAll(flyFixtureFile)
		Expect(err).NotTo(HaveOccurred())

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

			Expect(sess.Err).To(gbytes.Say(`name for the target must be specified \(--target/-t\)`))
		})
	})

	Context("status with target name", func() {
		Context("when target is saved with valid token", func() {
			It("command exist with 0", func() {
				flyCmd = exec.Command(flyPath, "-t", "another-test", "status")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))

				Expect(sess.Out).To(gbytes.Say(`logged in successfully`))
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
