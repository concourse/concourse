package integration_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/atc"
)

var _ = Describe("Fly CLI", func() {
	var (
		flyPath   string
		atcServer *ghttp.Server
	)

	BeforeEach(func() {
		var err error

		flyPath, err = gexec.Build("github.com/concourse/fly")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("checklist", func() {
		var (
			config atc.Config
			home   string
		)

		BeforeEach(func() {
			atcServer = ghttp.NewServer()

			var err error
			home, err = ioutil.TempDir("", "fly-home")
			Expect(err).NotTo(HaveOccurred())
			os.Setenv("HOME", home)

			flySaveCmd := exec.Command(flyPath, "save-target", "--api", atcServer.URL()+"/", "target-name")

			sess, err := gexec.Start(flySaveCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))

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
		})

		AfterEach(func() {
			err := os.RemoveAll(home)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when a pipeline name is not specified", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/pipelines/some-pipeline/config"),
						ghttp.RespondWithJSONEncoded(200, config, http.Header{atc.ConfigVersionHeader: {"42"}}),
					),
				)
			})

			It("prints the config as yaml to stdout", func() {
				flyCmd := exec.Command(flyPath, "-t", "target-name", "checklist", "-p", "some-pipeline")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))

				Expect(string(sess.Out.Contents())).To(Equal(fmt.Sprintf(
					`#- some-group
job-1: concourse.check %s   some-pipeline job-1
job-2: concourse.check %s   some-pipeline job-2

#- some-other-group
job-3: concourse.check %s   some-pipeline job-3
job-4: concourse.check %s   some-pipeline job-4

#- misc
some-orphaned-job: concourse.check %s   some-pipeline some-orphaned-job

`, atcServer.URL(), atcServer.URL(), atcServer.URL(), atcServer.URL(), atcServer.URL())))

			})
		})

		Context("when a pipeline name is specified", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/pipelines/some-pipeline/config"),
						ghttp.RespondWithJSONEncoded(200, config, http.Header{atc.ConfigVersionHeader: {"42"}}),
					),
				)
			})

			It("prints the config as yaml to stdout", func() {
				flyCmd := exec.Command(flyPath, "-t", "target-name", "checklist", "-p", "some-pipeline")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))

				Expect(string(sess.Out.Contents())).To(Equal(fmt.Sprintf(
					`#- some-group
job-1: concourse.check %s   some-pipeline job-1
job-2: concourse.check %s   some-pipeline job-2

#- some-other-group
job-3: concourse.check %s   some-pipeline job-3
job-4: concourse.check %s   some-pipeline job-4

#- misc
some-orphaned-job: concourse.check %s   some-pipeline some-orphaned-job

`, atcServer.URL(), atcServer.URL(), atcServer.URL(), atcServer.URL(), atcServer.URL())))
			})
		})
	})
})
