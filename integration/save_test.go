package integration_test

import (
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

var _ = Describe("Fly CLI", func() {
	var flyrc string
	var stockYAML = `
targets:
  some-target-name:
    api: some-existing-text
`

	BeforeEach(func() {
		tmpDir, _ := ioutil.TempDir("", "fly-test")
		os.Setenv("HOME", tmpDir)
		os.Setenv("HOMEPATH", tmpDir)
		os.Unsetenv("HOMEDRIVE")
		flyrc = filepath.Join(userHomeDir(), ".flyrc")
	})

	AfterEach(func() {
		theFile, _ := os.Open(flyrc)
		theFile.Close()
		os.Remove(flyrc)
		os.Unsetenv("HOME")
		os.Unsetenv("HOMEPATH")
	})

	It("should exit 1 when no name is provided", func() {
		flyCmd := exec.Command(flyPath, "save-target", "--api",
			"http://some-target", "--username", "some-username",
			"--password", "some-password", "--cert", "~/path/to/cert",
		)

		sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

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
			flyCmd := exec.Command(flyPath, "save-target", "--api",
				targetURL, "my-test-target",
			)

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))

			flyCmd = exec.Command(flyPath, "-t", targetURL, "checklist")

			sess, err = gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())

			<-sess.Exited
			Ω(sess.ExitCode()).Should(Equal(0))
		})
	})

	Context("when a .flyrc exists", func() {
		Context("and the target does not exist", func() {
			It("should append content to .flyrc", func() {
				err := ioutil.WriteFile(flyrc, []byte(stockYAML), os.ModePerm)
				Ω(err).ShouldNot(HaveOccurred())

				flyCmd := exec.Command(flyPath, "save-target", "--api",
					"http://some-target", "--username", "some-username",
					"--password", "some-password", "--cert", "~/path/to/cert",
					"some-update-target",
				)

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Ω(err).ShouldNot(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))
				Eventually(sess).Should(gbytes.Say("successfully saved target some-update-target\n"))

				flyrcBytes, err := ioutil.ReadFile(flyrc)
				Ω(err).ShouldNot(HaveOccurred())

				re := regexp.MustCompile("targets")
				Ω(re.FindAllString(string(flyrcBytes), -1)).Should(HaveLen(1))

				Ω(string(flyrcBytes)).To(ContainSubstring("some-existing-text"))
				Ω(string(flyrcBytes)).To(ContainSubstring("http://some-target"))
			})
		})

		Context("and the target is already saved", func() {
			It("should update the target", func() {
				flyCmd := exec.Command(flyPath, "save-target", "--api",
					"http://some-target", "--username", "some-username",
					"--password", "some-password", "--cert", "~/path/to/cert",
					"some-update-target",
				)

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Ω(err).ShouldNot(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				flyCmd = exec.Command(flyPath, "save-target", "--api",
					"http://a-different-target", "--username", "some-username",
					"--password", "stuff", "--cert", "~/path/to/different/cert",
					"some-update-target",
				)

				sess, err = gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Ω(err).ShouldNot(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				flyrcBytes, err := ioutil.ReadFile(flyrc)
				re := regexp.MustCompile("some-update-target")
				Ω(re.FindAllString(string(flyrcBytes), -1)).Should(HaveLen(1))
				Ω(string(flyrcBytes)).To(ContainSubstring("password: stuff"))
				Ω(string(flyrcBytes)).To(ContainSubstring("api: http://a-different-target"))
				Ω(string(flyrcBytes)).To(ContainSubstring("cert: ~/path/to/different/cert"))
			})
		})
	})

	Context("when no .flyrc exists", func() {
		It("should create the file and write the target", func() {
			flyCmd := exec.Command(flyPath, "save-target", "--api",
				"http://some-target", "--username", "some-username",
				"--password", "some-password", "--cert", "~/path/to/cert",
				"some-target",
			)

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(0))
			Eventually(sess).Should(gbytes.Say("successfully saved target some-target\n"))

			flyrcBytes, err := ioutil.ReadFile(flyrc)
			Ω(err).ShouldNot(HaveOccurred())

			type tempYAML struct {
				Targets map[string]yaml.MapSlice
			}

			var flyYAML *tempYAML
			yaml.Unmarshal(flyrcBytes, &flyYAML)

			Ω(flyYAML.Targets["some-target"]).To(ConsistOf([]yaml.MapItem{
				{Key: "api", Value: "http://some-target"},
				{Key: "username", Value: "some-username"},
				{Key: "password", Value: "some-password"},
				{Key: "cert", Value: "~/path/to/cert"},
			}))
		})
	})
})
