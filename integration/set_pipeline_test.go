package integration_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/gunk/urljoiner"
	"github.com/mgutz/ansi"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"
	"gopkg.in/yaml.v2"

	"github.com/concourse/atc"
)

func getConfigAndPausedState(r *http.Request) ([]byte, *bool) {
	defer r.Body.Close()

	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	Expect(err).NotTo(HaveOccurred())

	reader := multipart.NewReader(r.Body, params["boundary"])

	var payload []byte
	var state *bool

	yes := true
	no := false

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		Expect(err).NotTo(HaveOccurred())

		if part.FormName() == "paused" {
			pausedValue, readErr := ioutil.ReadAll(part)
			Expect(readErr).NotTo(HaveOccurred())

			if string(pausedValue) == "true" {
				state = &yes
			} else {
				state = &no
			}
		} else {
			payload, err = ioutil.ReadAll(part)
		}

		part.Close()
	}

	return payload, state
}

var _ = Describe("Fly CLI", func() {
	var (
		atcServer *ghttp.Server
	)

	Describe("set-pipeline", func() {
		var (
			config atc.Config
		)

		yes := func(stdin io.Writer) {
			fmt.Fprintf(stdin, "y\r")
		}

		no := func(stdin io.Writer) {
			fmt.Fprintf(stdin, "n\r")
		}

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

		Describe("templating", func() {
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

				path, err := atc.Routes.CreatePathForRoute(atc.GetConfig, rata.Params{"pipeline_name": "awesome-pipeline"})
				Expect(err).NotTo(HaveOccurred())

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", path),
						ghttp.RespondWithJSONEncoded(http.StatusOK, config, http.Header{atc.ConfigVersionHeader: {"42"}}),
					),
				)
			})

			Context("when configuring with templated keys succeeds", func() {
				BeforeEach(func() {
					path, err := atc.Routes.CreatePathForRoute(atc.SaveConfig, rata.Params{"pipeline_name": "awesome-pipeline"})
					Expect(err).NotTo(HaveOccurred())

					atcServer.RouteToHandler("PUT", path,
						ghttp.CombineHandlers(
							ghttp.VerifyHeaderKV(atc.ConfigVersionHeader, "42"),
							func(w http.ResponseWriter, r *http.Request) {
								bodyConfig, state := getConfigAndPausedState(r)
								Expect(state).To(BeTrue())

								receivedConfig := atc.Config{}
								err = yaml.Unmarshal(bodyConfig, &receivedConfig)
								Expect(err).NotTo(HaveOccurred())

								Expect(receivedConfig).To(Equal(config))

								w.WriteHeader(http.StatusNoContent)
							},
						),
					)
				})

				It("parses the config file and sends it to the ATC", func() {
					flyCmd := exec.Command(
						flyPath, "-t", atcServer.URL()+"/",
						"set-pipeline",
						"--pipeline", "awesome-pipeline",
						"-c", "fixtures/testConfig.yml",
						"--var", "resource-key=verysecret",
						"--load-vars-from", "fixtures/vars.yml",
					)

					stdin, err := flyCmd.StdinPipe()
					Expect(err).NotTo(HaveOccurred())

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
					yes(stdin)
					Eventually(sess).Should(gbytes.Say("configuration updated"))

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))

					Expect(atcServer.ReceivedRequests()).To(HaveLen(2))
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
				Expect(err).NotTo(HaveOccurred())

				changedConfig = config

				path, err := atc.Routes.CreatePathForRoute(atc.GetConfig, rata.Params{"pipeline_name": "awesome-pipeline"})
				Expect(err).NotTo(HaveOccurred())

				atcServer.RouteToHandler("GET", path,
					ghttp.RespondWithJSONEncoded(http.StatusOK, config, http.Header{atc.ConfigVersionHeader: {"42"}}),
				)
			})

			JustBeforeEach(func() {
				var err error

				payload, err = yaml.Marshal(changedConfig)
				Expect(err).NotTo(HaveOccurred())

				_, err = configFile.Write(payload)
				Expect(err).NotTo(HaveOccurred())

				err = configFile.Close()
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				err := os.RemoveAll(configFile.Name())
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when not specifying a pipeline name", func() {
				It("fails and says you should give a pipeline name", func() {
					flyCmd := exec.Command(flyPath, "-t", atcServer.URL()+"/", "set-pipeline", "-c", configFile.Name())

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))

					Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("p", "pipeline") + "' was not specified"))
				})
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

					path, err := atc.Routes.CreatePathForRoute(atc.SaveConfig, rata.Params{"pipeline_name": "awesome-pipeline"})
					Expect(err).NotTo(HaveOccurred())

					atcServer.RouteToHandler("PUT", path,
						ghttp.CombineHandlers(
							ghttp.VerifyHeaderKV(atc.ConfigVersionHeader, "42"),
							func(w http.ResponseWriter, r *http.Request) {
								config, state := getConfigAndPausedState(r)
								Expect(config).To(Equal(payload))
								Expect(*state).To(BeTrue(), "paused was not set in the request")
							},
							ghttp.RespondWith(http.StatusNoContent, ""),
						),
					)
				})

				It("parses the config file and sends it to the ATC", func() {
					flyCmd := exec.Command(flyPath, "-t", atcServer.URL()+"/", "set-pipeline", "-p", "awesome-pipeline", "-c", configFile.Name())

					stdin, err := flyCmd.StdinPipe()
					Expect(err).NotTo(HaveOccurred())

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

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

					Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
					yes(stdin)

					Eventually(sess).Should(gbytes.Say("configuration updated"))

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))

					Expect(atcServer.ReceivedRequests()).To(HaveLen(2))
				})

				It("bails if the user rejects the diff", func() {
					flyCmd := exec.Command(flyPath, "-t", atcServer.URL()+"/", "set-pipeline", "-p", "awesome-pipeline", "-c", configFile.Name())

					stdin, err := flyCmd.StdinPipe()
					Expect(err).NotTo(HaveOccurred())

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
					no(stdin)

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))

					Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
				})
			})

			Context("when configuring fails", func() {
				BeforeEach(func() {
					path, err := atc.Routes.CreatePathForRoute(atc.SaveConfig, rata.Params{"pipeline_name": "awesome-pipeline"})
					Expect(err).NotTo(HaveOccurred())

					atcServer.RouteToHandler("PUT", path,
						ghttp.RespondWith(http.StatusUnauthorized, "nope"),
					)
				})

				It("prints the error to stderr and exits 1", func() {
					flyCmd := exec.Command(flyPath, "-t", atcServer.URL()+"/", "set-pipeline", "-c", configFile.Name(), "-p", "awesome-pipeline")

					stdin, err := flyCmd.StdinPipe()
					Expect(err).NotTo(HaveOccurred())

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
					yes(stdin)

					Eventually(sess.Err).Should(gbytes.Say("failed to update configuration:"))
					Eventually(sess.Err).Should(gbytes.Say("401 Unauthorized"))
					Eventually(sess.Err).Should(gbytes.Say("nope"))

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))
				})
			})

			Context("when the server says this is the first time it's creating the pipeline", func() {
				BeforeEach(func() {
					path, err := atc.Routes.CreatePathForRoute(atc.SaveConfig, rata.Params{"pipeline_name": "awesome-pipeline"})
					Expect(err).NotTo(HaveOccurred())

					atcServer.RouteToHandler("PUT", path, ghttp.CombineHandlers(
						ghttp.VerifyHeaderKV(atc.ConfigVersionHeader, "42"),
						func(w http.ResponseWriter, r *http.Request) {
							config, state := getConfigAndPausedState(r)
							Expect(config).To(Equal(payload))
							Expect(state).To(BeTrue())
						},
						ghttp.RespondWith(http.StatusCreated, ""),
					))
				})

				It("succeeds and prints an error message to help the user", func() {
					flyCmd := exec.Command(flyPath, "-t", atcServer.URL()+"/", "set-pipeline", "-p", "awesome-pipeline", "-c", configFile.Name())

					stdin, err := flyCmd.StdinPipe()
					Expect(err).NotTo(HaveOccurred())

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
					yes(stdin)

					pipelineURL := urljoiner.Join(atcServer.URL(), "pipelines", "awesome-pipeline")

					Eventually(sess).Should(gbytes.Say("pipeline created!"))
					Eventually(sess).Should(gbytes.Say(fmt.Sprintf("you can view your pipeline here: %s", pipelineURL)))

					Eventually(sess).Should(gbytes.Say("the pipeline is currently paused. to unpause, either:"))
					Eventually(sess).Should(gbytes.Say("  - run 'fly -t [target] unpause-pipeline -p [pipeline]"))
					Eventually(sess).Should(gbytes.Say("  - click play next to the pipeline in the web ui"))

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))

					Expect(atcServer.ReceivedRequests()).To(HaveLen(2))
				})
			})

			Context("when the server rejects the request", func() {
				BeforeEach(func() {
					path, err := atc.Routes.CreatePathForRoute(atc.SaveConfig, rata.Params{"pipeline_name": "awesome-pipeline"})
					Expect(err).NotTo(HaveOccurred())

					atcServer.RouteToHandler("PUT", path, func(w http.ResponseWriter, r *http.Request) {
						atcServer.CloseClientConnections()
					})
				})

				It("prints the error to stderr and exits 1", func() {
					flyCmd := exec.Command(flyPath, "-t", atcServer.URL()+"/", "set-pipeline", "-c", configFile.Name(), "-p", "awesome-pipeline")

					stdin, err := flyCmd.StdinPipe()
					Expect(err).NotTo(HaveOccurred())

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
					yes(stdin)

					Eventually(sess.Err).Should(gbytes.Say("failed to update configuration: Put"))

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))
				})
			})
		})
	})
})
