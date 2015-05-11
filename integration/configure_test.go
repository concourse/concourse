package integration_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"

	"github.com/mgutz/ansi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"
	"gopkg.in/yaml.v2"

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
		Ω(err).ShouldNot(HaveOccurred())
	})

	Describe("configure", func() {
		var (
			config atc.Config
		)

		BeforeEach(func() {
			atcServer = ghttp.NewServer()

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

				Jobs: atc.JobConfigs{
					{
						Name: "some-job",

						Public: true,

						TaskConfigPath: "some/config/path.yml",
						TaskConfig: &atc.TaskConfig{
							Image: "some-image",
							Params: map[string]string{
								"A": "B",
							},
						},

						Privileged: true,

						Serial: true,

						InputConfigs: []atc.JobInputConfig{
							{
								RawName:  "some-input",
								Resource: "some-resource",
								Params: atc.Params{
									"some-param": "some-value",
								},
								Passed: []string{"job-1", "job-2"},
							},
						},

						OutputConfigs: []atc.JobOutputConfig{
							{
								Resource: "some-resource",
								Params: atc.Params{
									"some-param": "some-value",
								},
								RawPerformOn: []atc.Condition{"success", "failure"},
							},
						},
					},
					{
						Name: "some-other-job",

						TaskConfigPath: "some/config/path.yml",

						InputConfigs: []atc.JobInputConfig{
							{
								RawName:  "some-other-input",
								Resource: "some-other-resource",
								Params: atc.Params{
									"some-param": "some-value",
								},
								Passed: []string{"job-1", "job-2"},
							},
						},

						OutputConfigs: []atc.JobOutputConfig{
							{
								Resource: "some-other-resource",
								Params: atc.Params{
									"some-param": "some-value",
								},
								RawPerformOn: []atc.Condition{"success", "failure"},
							},
						},
					},
				},
			}
		})

		Describe("getting", func() {

			Context("when not specifying a pipeline name", func() {
				BeforeEach(func() {
					path, err := atc.Routes.CreatePathForRoute(atc.GetConfig, rata.Params{"pipeline_name": "main"})
					Ω(err).ShouldNot(HaveOccurred())

					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", path),
							ghttp.RespondWithJSONEncoded(200, config, http.Header{atc.ConfigVersionHeader: {"42"}}),
						),
					)
				})

				It("prints the config as yaml to stdout", func() {
					flyCmd := exec.Command(flyPath, "-t", atcServer.URL()+"/", "configure")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Ω(err).ShouldNot(HaveOccurred())

					<-sess.Exited
					Ω(sess.ExitCode()).Should(Equal(0))

					var printedConfig atc.Config
					err = yaml.Unmarshal(sess.Out.Contents(), &printedConfig)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(printedConfig).Should(Equal(config))
				})

				Context("when -j is given", func() {
					It("prints the config as json to stdout", func() {
						flyCmd := exec.Command(flyPath, "-t", atcServer.URL()+"/", "configure", "-j")

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Ω(err).ShouldNot(HaveOccurred())

						<-sess.Exited
						Ω(sess.ExitCode()).Should(Equal(0))

						var printedConfig atc.Config
						err = json.Unmarshal(sess.Out.Contents(), &printedConfig)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(printedConfig).Should(Equal(config))
					})
				})
			})

			Context("when specifying a pipeline name", func() {
				BeforeEach(func() {
					path, err := atc.Routes.CreatePathForRoute(atc.GetConfig, rata.Params{"pipeline_name": "some-pipeline"})
					Ω(err).ShouldNot(HaveOccurred())

					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", path),
							ghttp.RespondWithJSONEncoded(200, config, http.Header{atc.ConfigVersionHeader: {"42"}}),
						),
					)
				})

				It("prints the config as yaml to stdout", func() {
					flyCmd := exec.Command(flyPath, "-t", atcServer.URL()+"/", "configure", "some-pipeline")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Ω(err).ShouldNot(HaveOccurred())

					<-sess.Exited
					Ω(sess.ExitCode()).Should(Equal(0))

					var printedConfig atc.Config
					err = yaml.Unmarshal(sess.Out.Contents(), &printedConfig)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(printedConfig).Should(Equal(config))
				})

				Context("when -j is given", func() {
					It("prints the config as json to stdout", func() {
						flyCmd := exec.Command(flyPath, "-t", atcServer.URL()+"/", "configure", "some-pipeline", "-j")

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Ω(err).ShouldNot(HaveOccurred())

						<-sess.Exited
						Ω(sess.ExitCode()).Should(Equal(0))

						var printedConfig atc.Config
						err = json.Unmarshal(sess.Out.Contents(), &printedConfig)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(printedConfig).Should(Equal(config))
					})
				})
			})
		})

		Describe("templating", func() {
			var (
				payload []byte
			)

			BeforeEach(func() {
				config = atc.Config{
					Groups: atc.GroupConfigs{},
					Resources: atc.ResourceConfigs{
						{
							Name: "some-resource",
							Type: "template-type",
							Source: atc.Source{
								"source-config": "some-value",
							},
						},
						{
							Name: "some-other-resource",
							Type: "some-other-type",
							Source: atc.Source{
								"secret_key": "verysecret",
							},
						},
					},

					Jobs: atc.JobConfigs{},
				}

				path, err := atc.Routes.CreatePathForRoute(atc.GetConfig, rata.Params{"pipeline_name": "main"})
				Ω(err).ShouldNot(HaveOccurred())

				atcServer.RouteToHandler("GET", path,
					ghttp.RespondWithJSONEncoded(200, config, http.Header{atc.ConfigVersionHeader: {"42"}}),
				)
			})

			Context("when configuring with templated keys succeeds", func() {
				JustBeforeEach(func() {
					var err error
					payload, err = yaml.Marshal(config)
					Ω(err).ShouldNot(HaveOccurred())
				})

				BeforeEach(func() {
					path, err := atc.Routes.CreatePathForRoute(atc.SaveConfig, rata.Params{"pipeline_name": "main"})
					Ω(err).ShouldNot(HaveOccurred())

					atcServer.RouteToHandler("PUT", path,
						ghttp.CombineHandlers(
							ghttp.VerifyHeaderKV(atc.ConfigVersionHeader, "42"),
							ghttp.VerifyHeaderKV("Content-Type", "application/x-yaml"),
							func(w http.ResponseWriter, r *http.Request) {
								body, err := ioutil.ReadAll(r.Body)
								Ω(err).ShouldNot(HaveOccurred())

								receivedConfig := atc.Config{}

								err = yaml.Unmarshal(body, &receivedConfig)
								Ω(err).ShouldNot(HaveOccurred())

								Ω(receivedConfig).Should(Equal(config))
							},
							ghttp.RespondWith(200, ""),
						),
					)
				})

				It("parses the config file and sends it to the ATC", func() {
					flyCmd := exec.Command(
						flyPath, "-t", atcServer.URL()+"/",
						"configure",
						"-c", "fixtures/testConfig.yml",
						"-var", "resource-key=verysecret",
						"-vars-from", "fixtures/vars.yml",
					)

					stdin, err := flyCmd.StdinPipe()
					Ω(err).ShouldNot(HaveOccurred())

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Ω(err).ShouldNot(HaveOccurred())

					Eventually(sess).Should(gbytes.Say(`apply configuration\? \(y/n\): `))
					fmt.Fprintln(stdin, "y")
					Eventually(sess).Should(gbytes.Say("configuration updated"))

					<-sess.Exited
					Ω(sess.ExitCode()).Should(Equal(0))

					Ω(atcServer.ReceivedRequests()).Should(HaveLen(2))
				})
			})

		})

		Describe("setting", func() {
			var (
				changedConfig atc.Config

				payload    []byte
				configFile *os.File
			)

			BeforeEach(func() {
				var err error

				configFile, err = ioutil.TempFile("", "fly-config-file")
				Ω(err).ShouldNot(HaveOccurred())

				changedConfig = config

				path, err := atc.Routes.CreatePathForRoute(atc.GetConfig, rata.Params{"pipeline_name": "main"})
				Ω(err).ShouldNot(HaveOccurred())

				atcServer.RouteToHandler("GET", path,
					ghttp.RespondWithJSONEncoded(200, config, http.Header{atc.ConfigVersionHeader: {"42"}}),
				)
			})

			JustBeforeEach(func() {
				var err error

				payload, err = yaml.Marshal(changedConfig)
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
					newGroup := changedConfig.Groups[1]
					newGroup.Name = "some-new-group"
					changedConfig.Groups[0].Jobs = append(changedConfig.Groups[0].Jobs, "some-new-job")
					changedConfig.Groups = append(changedConfig.Groups[:1], newGroup)

					newResource := changedConfig.Resources[1]
					newResource.Name = "some-new-resource"
					changedConfig.Resources[0].Type = "some-new-type"
					changedConfig.Resources = append(changedConfig.Resources[:1], newResource)

					newJob := changedConfig.Jobs[1]
					newJob.Name = "some-new-job"
					changedConfig.Jobs[0].Serial = false
					changedConfig.Jobs = append(changedConfig.Jobs[:1], newJob)

					path, err := atc.Routes.CreatePathForRoute(atc.SaveConfig, rata.Params{"pipeline_name": "main"})
					Ω(err).ShouldNot(HaveOccurred())

					atcServer.RouteToHandler("PUT", path,
						ghttp.CombineHandlers(
							ghttp.VerifyHeaderKV(atc.ConfigVersionHeader, "42"),
							ghttp.VerifyHeaderKV("Content-Type", "application/x-yaml"),
							func(w http.ResponseWriter, r *http.Request) {
								Ω(ioutil.ReadAll(r.Body)).Should(Equal(payload))
							},
							ghttp.RespondWith(200, ""),
						),
					)
				})

				It("parses the config file and sends it to the ATC", func() {
					flyCmd := exec.Command(flyPath, "-t", atcServer.URL()+"/", "configure", "-c", configFile.Name())

					stdin, err := flyCmd.StdinPipe()
					Ω(err).ShouldNot(HaveOccurred())

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Ω(err).ShouldNot(HaveOccurred())

					Eventually(sess).Should(gbytes.Say("group some-group has changed"))
					Eventually(sess.Out.Contents).Should(ContainSubstring(ansi.Color("- some-new-job", "green")))

					Eventually(sess).Should(gbytes.Say("group some-other-group has been removed"))
					Eventually(sess.Out.Contents).Should(ContainSubstring(ansi.Color("name: some-other-group", "red")))

					Eventually(sess).Should(gbytes.Say("group some-new-group has been added"))
					Eventually(sess.Out.Contents).Should(ContainSubstring(ansi.Color("name: some-new-group", "green")))

					Eventually(sess).Should(gbytes.Say("resource some-resource has changed"))
					Eventually(sess.Out.Contents).Should(ContainSubstring(ansi.Color("type: some-type", "red")))
					Eventually(sess.Out.Contents).Should(ContainSubstring(ansi.Color("type: some-new-type", "green")))

					Eventually(sess).Should(gbytes.Say("resource some-other-resource has been removed"))
					Eventually(sess.Out.Contents).Should(ContainSubstring(ansi.Color("name: some-other-resource", "red")))

					Eventually(sess).Should(gbytes.Say("resource some-new-resource has been added"))
					Eventually(sess.Out.Contents).Should(ContainSubstring(ansi.Color("name: some-new-resource", "green")))

					Eventually(sess).Should(gbytes.Say("job some-job has changed"))
					Eventually(sess.Out.Contents).Should(ContainSubstring(ansi.Color("serial: true", "red")))

					Eventually(sess).Should(gbytes.Say("job some-other-job has been removed"))
					Eventually(sess.Out.Contents).Should(ContainSubstring(ansi.Color("name: some-other-job", "red")))

					Eventually(sess).Should(gbytes.Say("job some-new-job has been added"))
					Eventually(sess.Out.Contents).Should(ContainSubstring(ansi.Color("name: some-new-job", "green")))

					Eventually(sess).Should(gbytes.Say(`apply configuration\? \(y/n\): `))
					fmt.Fprintln(stdin, "y")

					Eventually(sess).Should(gbytes.Say("configuration updated"))

					<-sess.Exited
					Ω(sess.ExitCode()).Should(Equal(0))

					Ω(atcServer.ReceivedRequests()).Should(HaveLen(2))
				})

				It("bails if the user rejects the diff", func() {
					flyCmd := exec.Command(flyPath, "-t", atcServer.URL()+"/", "configure", "-c", configFile.Name())

					stdin, err := flyCmd.StdinPipe()
					Ω(err).ShouldNot(HaveOccurred())

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Ω(err).ShouldNot(HaveOccurred())

					Eventually(sess).Should(gbytes.Say(`apply configuration\? \(y/n\): `))
					fmt.Fprintln(stdin, "n")

					<-sess.Exited
					Ω(sess.ExitCode()).Should(Equal(1))

					Ω(atcServer.ReceivedRequests()).Should(HaveLen(1))
				})
			})

			Context("when configuring fails", func() {
				BeforeEach(func() {
					path, err := atc.Routes.CreatePathForRoute(atc.SaveConfig, rata.Params{"pipeline_name": "main"})
					Ω(err).ShouldNot(HaveOccurred())

					atcServer.RouteToHandler("PUT", path,
						ghttp.RespondWith(400, "nope"),
					)
				})

				It("prints the error to stderr and exits 1", func() {
					flyCmd := exec.Command(flyPath, "-t", atcServer.URL()+"/", "configure", "-c", configFile.Name())

					stdin, err := flyCmd.StdinPipe()
					Ω(err).ShouldNot(HaveOccurred())

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Ω(err).ShouldNot(HaveOccurred())

					Eventually(sess).Should(gbytes.Say(`apply configuration\? \(y/n\): `))
					fmt.Fprintln(stdin, "y")

					Eventually(sess.Err).Should(gbytes.Say("failed to update configuration."))
					Eventually(sess.Err).Should(gbytes.Say("  response code: 400"))
					Eventually(sess.Err).Should(gbytes.Say("  response body:"))
					Eventually(sess.Err).Should(gbytes.Say("    nope"))

					<-sess.Exited
					Ω(sess.ExitCode()).Should(Equal(1))
				})
			})

			Context("when the server rejects the request", func() {
				BeforeEach(func() {
					path, err := atc.Routes.CreatePathForRoute(atc.SaveConfig, rata.Params{"pipeline_name": "main"})
					Ω(err).ShouldNot(HaveOccurred())

					atcServer.RouteToHandler("PUT", path, func(w http.ResponseWriter, r *http.Request) {
						atcServer.CloseClientConnections()
					})
				})

				It("prints the error to stderr and exits 1", func() {
					flyCmd := exec.Command(flyPath, "-t", atcServer.URL()+"/", "configure", "-c", configFile.Name())

					stdin, err := flyCmd.StdinPipe()
					Ω(err).ShouldNot(HaveOccurred())

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Ω(err).ShouldNot(HaveOccurred())

					Eventually(sess).Should(gbytes.Say(`apply configuration\? \(y/n\): `))
					fmt.Fprintln(stdin, "y")

					Eventually(sess.Err).Should(gbytes.Say("failed to update configuration: Put"))

					<-sess.Exited
					Ω(sess.ExitCode()).Should(Equal(1))
				})
			})
		})
	})
})
