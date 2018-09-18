package integration_test

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/atc"
)

var _ = Describe("Fly CLI", func() {
	Describe("checklist", func() {
		var (
			config atc.Config
			home   string
		)

		BeforeEach(func() {
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
			It("errors", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "checklist")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
			})
		})

		Context("when specifying a pipeline name with a '/' character in it", func() {
			It("fails and says '/' characters are not allowed", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "checklist", "-p", "forbidden/pipelinename")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say("error: pipeline name cannot contain '/'"))
			})
		})

		Context("when a pipeline name is specified", func() {
			JustBeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/some-pipeline/config"),
						ghttp.RespondWithJSONEncoded(200, atc.ConfigResponse{Config: &config}, http.Header{atc.ConfigVersionHeader: {"42"}}),
					),
				)
			})

			Context("when there are groups", func() {
				It("prints the config as yaml to stdout", func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "checklist", "-p", "some-pipeline")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))

					Expect(string(sess.Out.Contents())).To(Equal(fmt.Sprintf(
						`#- some-group
job-1: concourse.check %s main some-pipeline job-1
job-2: concourse.check %s main some-pipeline job-2

#- some-other-group
job-3: concourse.check %s main some-pipeline job-3
job-4: concourse.check %s main some-pipeline job-4

#- misc
some-orphaned-job: concourse.check %s main some-pipeline some-orphaned-job

`, atcServer.URL(), atcServer.URL(), atcServer.URL(), atcServer.URL(), atcServer.URL())))
				})
			})

			Context("when there are no groups", func() {
				BeforeEach(func() {
					config = atc.Config{
						Jobs: atc.JobConfigs{
							{
								Name: "job-1",
							},
							{
								Name: "job-2",
							},
						},
					}

				})

				It("prints the config as yaml to stdout, and uses the pipeline name as header", func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "checklist", "-p", "some-pipeline")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))

					Expect(string(sess.Out.Contents())).To(Equal(fmt.Sprintf(
						`#- some-pipeline
job-1: concourse.check %s main some-pipeline job-1
job-2: concourse.check %s main some-pipeline job-2

`, atcServer.URL(), atcServer.URL())))
				})
			})
		})
	})
})
