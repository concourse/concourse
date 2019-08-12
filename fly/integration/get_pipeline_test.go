package integration_test

import (
	"encoding/json"
	"net/http"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/ghodss/yaml"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"

	"github.com/concourse/concourse/atc"
)

var _ = Describe("Fly CLI", func() {
	Describe("get-pipeline", func() {
		var (
			config atc.Config
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

				Resources: atc.ResourceConfigs{
					{
						Name: "some-resource",
						Type: "some-type",
						Source: atc.Source{
							"source-config": "some-value",
						},
					},
					{
						Name: "some-other-resource",
						Type: "some-other-type",
						Source: atc.Source{
							"source-config": "some-value",
						},
					},
				},

				ResourceTypes: atc.ResourceTypes{
					{
						Name: "some-resource-type",
						Type: "some-type",
						Source: atc.Source{
							"source-config": "some-value",
						},
					},
					{
						Name: "some-other-resource-type",
						Type: "some-other-type",
						Source: atc.Source{
							"source-config": "some-value",
						},
					},
				},

				Jobs: atc.JobConfigs{
					{
						Name:   "some-job",
						Public: true,
						Serial: true,
					},
					{
						Name: "some-other-job",
					},
				},
			}
		})

		Describe("getting", func() {
			Context("when not specifying a pipeline name", func() {
				It("fails and says you should give a pipeline name", func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "get-pipeline")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))

					Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("p", "pipeline") + "' was not specified"))
				})
			})

			Context("when specifying a pipeline name with a '/' character in it", func() {
				It("fails and says '/' characters are not allowed", func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "get-pipeline", "-p", "forbidden/pipelinename")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))

					Expect(sess.Err).To(gbytes.Say("error: pipeline name cannot contain '/'"))
				})
			})

			Context("when specifying a pipeline name", func() {
				var path string
				BeforeEach(func() {
					var err error
					path, err = atc.Routes.CreatePathForRoute(atc.GetConfig, rata.Params{"pipeline_name": "some-pipeline", "team_name": "main"})
					Expect(err).NotTo(HaveOccurred())
				})

				Context("and pipeline is not found", func() {
					JustBeforeEach(func() {
						atcServer.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("GET", path),
								ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
							),
						)
					})

					It("should print pipeline not found error", func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "get-pipeline", "--pipeline", "some-pipeline", "-j")

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(1))

						Expect(sess.Err).To(gbytes.Say("error: pipeline not found"))
					})
				})

				Context("when atc returns valid config", func() {
					BeforeEach(func() {
						atcServer.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("GET", path),
								ghttp.RespondWithJSONEncoded(200, atc.ConfigResponse{Config: config}, http.Header{atc.ConfigVersionHeader: {"42"}}),
							),
						)
					})

					It("prints the config as yaml to stdout", func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "get-pipeline", "--pipeline", "some-pipeline")

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))

						var printedConfig atc.Config
						err = yaml.Unmarshal(sess.Out.Contents(), &printedConfig)
						Expect(err).NotTo(HaveOccurred())

						Expect(printedConfig).To(Equal(config))
					})

					Context("when -j is given", func() {
						It("prints the config as json to stdout", func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "get-pipeline", "--pipeline", "some-pipeline", "-j")

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(0))

							var printedConfig atc.Config
							err = json.Unmarshal(sess.Out.Contents(), &printedConfig)
							Expect(err).NotTo(HaveOccurred())

							Expect(printedConfig).To(Equal(config))
						})
					})
				})
			})
		})
	})
})
