package integration_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"gopkg.in/yaml.v2"

	"github.com/concourse/atc"
	"github.com/concourse/turbine"
)

var _ = Describe("Fly CLI", func() {
	var (
		flyPath   string
		atcServer *ghttp.Server
	)

	BeforeEach(func() {
		var err error

		flyPath, err = gexec.Build("github.com/concourse/fly")
		Ω(err).ShouldNot(HaveOccurred())
	})

	Describe("configure", func() {
		var (
			config atc.Config
		)

		BeforeEach(func() {
			atcServer = ghttp.NewServer()

			os.Setenv("ATC_URL", atcServer.URL())

			config = atc.Config{
				Groups: atc.GroupConfigs{
					{
						Name:      "some-group",
						Jobs:      []string{"job-1", "job-2"},
						Resources: []string{"resource-1", "resource-2"},
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
				},

				Jobs: atc.JobConfigs{
					{
						Name: "some-job",

						Public: true,

						BuildConfigPath: "some/config/path.yml",
						BuildConfig: turbine.Config{
							Image: "some-image",
							Params: map[string]string{
								"A": "B",
							},
						},

						Privileged: true,

						Serial: true,

						Inputs: []atc.InputConfig{
							{
								Name:     "some-input",
								Resource: "some-resource",
								Params: atc.Params{
									"some-param": "some-value",
								},
								Passed: []string{"job-1", "job-2"},
							},
						},

						Outputs: []atc.OutputConfig{
							{
								Resource: "some-resource",
								Params: atc.Params{
									"some-param": "some-value",
								},
								PerformOn: []atc.OutputCondition{"success", "failure"},
							},
						},
					},
				},
			}
		})

		Describe("getting", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/config"),
						ghttp.RespondWithJSONEncoded(200, config),
					),
				)
			})

			It("prints the config as yaml to stdout", func() {
				flyCmd := exec.Command(flyPath, "configure")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Ω(err).ShouldNot(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))

				var printedConfig atc.Config
				err = yaml.Unmarshal(sess.Out.Contents(), &printedConfig)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(printedConfig).Should(Equal(config))
			})

			Context("when -j is given", func() {
				It("prints the config as json to stdout", func() {
					flyCmd := exec.Command(flyPath, "configure", "-j")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Ω(err).ShouldNot(HaveOccurred())

					Eventually(sess).Should(gexec.Exit(0))

					var printedConfig atc.Config
					err = json.Unmarshal(sess.Out.Contents(), &printedConfig)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(printedConfig).Should(Equal(config))
				})
			})
		})

		Describe("setting", func() {
			var (
				configFile *os.File
			)

			BeforeEach(func() {
				var err error

				configFile, err = ioutil.TempFile("", "fly-config-file")
				Ω(err).ShouldNot(HaveOccurred())

				payload, err := yaml.Marshal(config)
				Ω(err).ShouldNot(HaveOccurred())

				_, err = configFile.Write(payload)
				Ω(err).ShouldNot(HaveOccurred())

				err = configFile.Close()
				Ω(err).ShouldNot(HaveOccurred())
			})

			AfterEach(func() {
				err := os.RemoveAll(configFile.Name())
				Ω(err).ShouldNot(HaveOccurred())
			})

			Context("when configuring succeeds", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", "/api/v1/config"),
							ghttp.VerifyJSONRepresenting(config),
							ghttp.RespondWith(200, ""),
						),
					)
				})

				It("parses the config file and sends it to the ATC", func() {
					flyCmd := exec.Command(flyPath, "configure", "-c", configFile.Name())

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Ω(err).ShouldNot(HaveOccurred())

					Eventually(sess).Should(gexec.Exit(0))

					Ω(atcServer.ReceivedRequests()).Should(HaveLen(1))
				})
			})

			Context("when configuring fails", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", "/api/v1/config"),
							ghttp.VerifyJSONRepresenting(config),
							ghttp.RespondWith(400, "nope"),
						),
					)
				})

				It("prints the error to stderr and exits 1", func() {
					flyCmd := exec.Command(flyPath, "configure", "-c", configFile.Name())

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Ω(err).ShouldNot(HaveOccurred())

					Eventually(sess.Err).Should(gbytes.Say("nope"))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})
		})
	})
})
