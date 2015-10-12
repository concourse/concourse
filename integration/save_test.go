package integration_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	"gopkg.in/yaml.v2"

	"github.com/concourse/atc"
)

var _ = Describe("save-target Command", func() {
	var tmpDir string
	var flyrc string
	var stockYAML = `
targets:
  some-target-name:
    api: some-existing-text
`
	var certififatePath string

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "fly-test")
		Expect(err).NotTo(HaveOccurred())

		os.Setenv("HOME", tmpDir)
		os.Setenv("HOMEPATH", tmpDir)
		os.Unsetenv("HOMEDRIVE")

		flyrc = filepath.Join(userHomeDir(), ".flyrc")
		certififatePath = filepath.Join(tmpDir, "fly.cert")

		err = ioutil.WriteFile(certififatePath, []byte("a really secure certificate"), 0600)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)

		os.Unsetenv("HOME")
		os.Unsetenv("HOMEPATH")
	})

	It("should exit 1 when no name is provided", func() {
		flyCmd := exec.Command(
			flyPath,
			"save-target",
			"--api", "http://some-target",
			"--username", "some-username",
			"--password", "some-password",
			"--cert", certififatePath,
		)

		sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(sess).Should(gexec.Exit(1))
	})

	Context("when saving a target", func() {
		var config atc.Config
		var targetURL string

		BeforeEach(func() {
			atcServer := ghttp.NewServer()

			targetURL = atcServer.URL()

			config = atc.Config{
				Groups: atc.GroupConfigs{
					{
						Name:      "some-group",
						Jobs:      []string{"job-1", "job-2"},
						Resources: []string{"resource-1", "resource-2"},
					},
					{
						Name:      "some-other-group",
						Jobs:      []string{"job-3", "job-4"},
						Resources: []string{"resource-6", "resource-4"},
					},
				},

				Jobs: atc.JobConfigs{
					{
						Name: "some-orphaned-job",
					},
				},
			}

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/pipelines/main/config"),
					ghttp.RespondWithJSONEncoded(200, config, http.Header{atc.ConfigVersionHeader: {"42"}}),
				),
			)
		})

		It("should use the target when passed to the next command", func() {
			flyCmd := exec.Command(flyPath,
				"save-target",
				"--api", targetURL,
				"--name", "my-test-target",
			)

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))

			flyCmd = exec.Command(
				flyPath,
				"-t", "my-test-target",
				"checklist",
				"-p", "main",
			)

			sess, err = gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))
		})

		It("should error when saving a target who's name begins with 'http'", func() {
			flyCmd := exec.Command(flyPath,
				"save-target",
				"--api", targetURL,
				"--name", "http://my-test-target",
			)

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(1))
			Expect(sess.Err.Contents()).To(MatchRegexp("target name.*http"))
		})
	})

	Context("when a .flyrc exists", func() {
		Context("and the target does not exist", func() {
			It("should append content to .flyrc", func() {
				err := ioutil.WriteFile(flyrc, []byte(stockYAML), os.ModePerm)
				Expect(err).NotTo(HaveOccurred())

				flyCmd := exec.Command(
					flyPath,
					"save-target",
					"--api", "http://some-target",
					"--username", "some-username",
					"--password", "some-password",
					"--cert", certififatePath,
					"--name", "some-update-target",
				)

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))
				Eventually(sess).Should(gbytes.Say("successfully saved target some-update-target\n"))

				flyrcBytes, err := ioutil.ReadFile(flyrc)
				Expect(err).NotTo(HaveOccurred())

				re := regexp.MustCompile("targets")
				Expect(re.FindAllString(string(flyrcBytes), -1)).To(HaveLen(1))

				Expect(string(flyrcBytes)).To(ContainSubstring("some-existing-text"))
				Expect(string(flyrcBytes)).To(ContainSubstring("http://some-target"))
			})
		})

		Context("and the target is already saved", func() {
			var updatedCertPath string

			BeforeEach(func() {
				updatedCertPath := filepath.Join(tmpDir, "updated-fly.cert")
				err := ioutil.WriteFile(updatedCertPath, []byte("a new really secure certificate"), 0600)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should update the target", func() {
				flyCmd := exec.Command(
					flyPath,
					"save-target",
					"--api", "http://some-target",
					"--username", "some-username",
					"--password", "some-password",
					"--cert", certififatePath,
					"--name", "some-update-target",
				)

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				flyCmd = exec.Command(
					flyPath,
					"save-target",
					"--api", "http://a-different-target",
					"--username", "some-username",
					"--password", "stuff",
					"--cert", updatedCertPath,
					"--name", "some-update-target",
				)

				sess, err = gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				flyrcBytes, err := ioutil.ReadFile(flyrc)
				re := regexp.MustCompile("some-update-target")
				Expect(re.FindAllString(string(flyrcBytes), -1)).To(HaveLen(1))
				Expect(string(flyrcBytes)).To(ContainSubstring("password: stuff"))
				Expect(string(flyrcBytes)).To(ContainSubstring("api: http://a-different-target"))
				Expect(string(flyrcBytes)).To(ContainSubstring(fmt.Sprintf("cert: %s", updatedCertPath)))
			})
		})
	})

	Context("when no .flyrc exists", func() {
		It("should create the file and write the target", func() {
			flyCmd := exec.Command(
				flyPath,
				"save-target",
				"--api", "http://some-target",
				"--username", "some-username",
				"--password", "some-password",
				"--cert", certififatePath,
				"--name", "some-target",
			)

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(0))
			Eventually(sess).Should(gbytes.Say("successfully saved target some-target\n"))

			flyrcBytes, err := ioutil.ReadFile(flyrc)
			Expect(err).NotTo(HaveOccurred())

			type tempYAML struct {
				Targets map[string]yaml.MapSlice
			}

			var flyYAML *tempYAML
			yaml.Unmarshal(flyrcBytes, &flyYAML)

			Expect(flyYAML.Targets["some-target"]).To(ConsistOf([]yaml.MapItem{
				{Key: "api", Value: "http://some-target"},
				{Key: "username", Value: "some-username"},
				{Key: "password", Value: "some-password"},
				{Key: "cert", Value: certififatePath},
			}))

		})
	})
})
