package integration_test

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mgutz/ansi"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"
	"sigs.k8s.io/yaml"

	"github.com/concourse/concourse/atc"
)

var _ = Describe("Fly CLI", func() {
	Describe("set-pipeline", func() {
		var (
			config atc.Config
		)

		yes := func(stdin io.Writer) {
			fmt.Fprintf(stdin, "y\n")
		}

		no := func(stdin io.Writer) {
			fmt.Fprintf(stdin, "n\n")
		}

		expectSaveConfigWithRef := func(ref atc.PipelineRef, config atc.Config) {
			path, err := atc.Routes.CreatePathForRoute(atc.SaveConfig, rata.Params{"pipeline_name": ref.Name, "team_name": "main"})
			Expect(err).NotTo(HaveOccurred())

			atcServer.RouteToHandler("PUT", path,
				ghttp.CombineHandlers(
					ghttp.VerifyHeaderKV(atc.ConfigVersionHeader, "42"),
					func(w http.ResponseWriter, r *http.Request) {
						for k, v := range ref.QueryParams() {
							Expect(r.URL.Query()[k]).To(Equal(v))
						}
						bodyConfig := getConfig(r)

						receivedConfig := atc.Config{}
						err = yaml.Unmarshal(bodyConfig, &receivedConfig)
						Expect(err).NotTo(HaveOccurred())

						Expect(receivedConfig).To(Equal(config))

						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{}`))
					},
				),
			)

			path_get, err := atc.Routes.CreatePathForRoute(atc.GetPipeline, rata.Params{"pipeline_name": ref.Name, "team_name": "main"})
			Expect(err).NotTo(HaveOccurred())

			atcServer.RouteToHandler("GET", path_get,
				ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Pipeline{Name: ref.Name, InstanceVars: ref.InstanceVars, Paused: false, TeamName: "main"}),
			)
		}

		expectSaveConfig := func(config atc.Config) {
			expectSaveConfigWithRef(atc.PipelineRef{Name: "awesome-pipeline"}, config)
		}

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
					{
						Name: "some-resource-with-int-field",
						Type: "some-type",
						Source: atc.Source{
							"source-config": 5,
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
						Name: "some-unchanged-job",
					},
					{
						Name: "some-other-job",
					},
					{
						Name: "pinned-resource-job",
						PlanSequence: []atc.Step{
							{
								Config: &atc.GetStep{
									Name: "some-resource",
									Version: &atc.VersionConfig{
										Pinned: atc.Version{
											"ref": "some-ref",
										},
									},
								},
							},
						},
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

				path, err := atc.Routes.CreatePathForRoute(atc.GetConfig, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
				Expect(err).NotTo(HaveOccurred())

				path_get, err := atc.Routes.CreatePathForRoute(atc.GetPipeline, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
				Expect(err).NotTo(HaveOccurred())

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", path),
						ghttp.RespondWithJSONEncoded(http.StatusOK, atc.ConfigResponse{Config: config}, http.Header{atc.ConfigVersionHeader: {"42"}}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", path_get),
						ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Pipeline{Name: "awesome-pipeline", Paused: false, TeamName: "main"}),
					),
				)
			})

			Context("when configuring container limits in task", func() {
				It("succeeds", func() {
					flyCmd := exec.Command(
						flyPath, "-t", targetName,
						"set-pipeline",
						"--pipeline", "awesome-pipeline",
						"-c", "fixtures/testConfigContainerLimits.yml",
					)
					stdin, err := flyCmd.StdinPipe()
					Expect(err).NotTo(HaveOccurred())

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gbytes.Say(`cpu: 1024`))
					Eventually(sess).Should(gbytes.Say(`memory: 2147483648`))
					Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
					no(stdin)

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))
				})
			})

			Context("when configuring with old-style templated value that fails", func() {
				It("shows helpful error messages", func() {
					flyCmd := exec.Command(
						flyPath, "-t", targetName,
						"set-pipeline",
						"--pipeline", "awesome-pipeline",
						"-c", "fixtures/testConfig.yml",
					)

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Err).Should(gbytes.Say(`2 errors occurred:`))
					Eventually(sess.Err).Should(gbytes.Say(`\* unbound variable in template: 'resource-type'`))
					Eventually(sess.Err).Should(gbytes.Say(`\* unbound variable in template: 'resource-key'`))

					<-sess.Exited
					Expect(sess.ExitCode()).NotTo(Equal(0))
				})
			})

			Context("when configuring with old-style templated value succeeds", func() {
				BeforeEach(func() {
					expectSaveConfig(atc.Config{
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
									"secret_key": "overridden-secret",
								},
							},
						},

						Jobs: atc.JobConfigs{},
					})
				})

				It("parses the config file and sends it to the ATC", func() {
					Expect(func() {
						flyCmd := exec.Command(
							flyPath, "-t", targetName,
							"set-pipeline",
							"--pipeline", "awesome-pipeline",
							"-c", "fixtures/testConfig.yml",
							"--var", "resource-key=overridden-secret",
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
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(5))
				})

				Context("when a non-stringy var is specified with -v", func() {
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
										"secret_key": `{"complicated": "secret"}`,
									},
								},
							},

							Jobs: atc.JobConfigs{},
						}

						path, err := atc.Routes.CreatePathForRoute(atc.SaveConfig, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
						Expect(err).NotTo(HaveOccurred())

						atcServer.RouteToHandler("PUT", path,
							ghttp.CombineHandlers(
								ghttp.VerifyHeaderKV(atc.ConfigVersionHeader, "42"),
								func(w http.ResponseWriter, r *http.Request) {
									bodyConfig := getConfig(r)

									receivedConfig := atc.Config{}
									err = yaml.Unmarshal(bodyConfig, &receivedConfig)
									Expect(err).NotTo(HaveOccurred())

									Expect(receivedConfig).To(Equal(config))
									w.Header().Set("Content-Type", "application/json")

									w.WriteHeader(http.StatusOK)
									w.Write([]byte(`{}`))
								},
							),
						)
					})

					It("succeeds", func() {
						Expect(func() {
							flyCmd := exec.Command(
								flyPath, "-t", targetName,
								"set-pipeline",
								"-n",
								"--pipeline", "awesome-pipeline",
								"-c", "fixtures/testConfig.yml",
								"--var", `resource-key={"complicated": "secret"}`,
								"--load-vars-from", "fixtures/vars.yml",
							)

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())
							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(0))
						}).To(Change(func() int {
							return len(atcServer.ReceivedRequests())
						}).By(5))
					})
				})

				Context("when the --non-interactive is passed", func() {
					It("parses the config file and sends it to the ATC without interaction", func() {
						Expect(func() {
							flyCmd := exec.Command(
								flyPath, "-t", targetName,
								"set-pipeline",
								"--pipeline", "awesome-pipeline",
								"-c", "fixtures/testConfig.yml",
								"--var", "resource-key=overridden-secret",
								"--load-vars-from", "fixtures/vars.yml",
								"--non-interactive",
							)

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess).Should(gbytes.Say("configuration updated"))

							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(0))

						}).To(Change(func() int {
							return len(atcServer.ReceivedRequests())
						}).By(5))
					})
				})
			})

			Context("when a var is specified with -v", func() {
				BeforeEach(func() {
					expectSaveConfig(atc.Config{
						Resources: atc.ResourceConfigs{
							{
								Name: "some-resource",
								Type: "some-type",
								Tags: atc.Tags{"val-1", "val-2"},
								Source: atc.Source{
									"private_key": `-----BEGIN SOME KEY-----
this is super secure
-----END SOME KEY-----
`,
									"config-a": "some-param-a",
									"config-b": "some-param-b-via-v",
									"bool":     true,
									"number":   1.23,
								},
							},
						},

						Jobs: atc.JobConfigs{
							{
								Name: "some-job",
								PlanSequence: []atc.Step{
									{
										Config: &atc.GetStep{
											Name: "some-resource",
										},
									},
								},
							},
						},
					})
				})

				It("succeeds", func() {
					Expect(func() {
						flyCmd := exec.Command(
							flyPath, "-t", targetName,
							"set-pipeline",
							"-n",
							"--pipeline", "awesome-pipeline",
							"-c", "fixtures/vars-pipeline.yml",
							"-l", "fixtures/vars-pipeline-params-a.yml",
							"-l", "fixtures/vars-pipeline-params-types.yml",
							"-v", "param-b=some-param-b-via-v",
						)

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())
						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(5))
				})
			})

			Context("when vars are overridden with -v, some with special types", func() {
				BeforeEach(func() {
					expectSaveConfig(atc.Config{
						Resources: atc.ResourceConfigs{
							{
								Name: "some-resource",
								Type: "some-type",
								Tags: atc.Tags{"val-1", "val-2"},
								Source: atc.Source{
									"private_key": `-----BEGIN SOME KEY-----
this is super secure
-----END SOME KEY-----
`,
									"config-a": "some-param-a",
									"config-b": "some\nmultiline\nbusiness\n",
									"bool":     false,
									"number":   3.14,
								},
							},
						},

						Jobs: atc.JobConfigs{
							{
								Name: "some-job",
								PlanSequence: []atc.Step{
									{
										Config: &atc.GetStep{
											Name: "some-resource",
										},
									},
								},
							},
						},
					})
				})

				It("succeeds", func() {
					Expect(func() {
						flyCmd := exec.Command(
							flyPath, "-t", targetName,
							"set-pipeline",
							"-n",
							"--pipeline", "awesome-pipeline",
							"-c", "fixtures/vars-pipeline.yml",
							"-l", "fixtures/vars-pipeline-params-a.yml",
							"-l", "fixtures/vars-pipeline-params-b.yml",
							"-l", "fixtures/vars-pipeline-params-types.yml",
							"-v", "param-b=some\nmultiline\nbusiness\n",
							"-y", "bool-param=false",
							"-y", "number-param=3.14",
						)

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())
						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(5))
				})
			})

			Context("when an instance var is specified with -i", func() {
				BeforeEach(func() {
					expectSaveConfigWithRef(atc.PipelineRef{
						Name:         "awesome-pipeline",
						InstanceVars: atc.InstanceVars{"param-b": "some-param-b-via-i"},
					}, atc.Config{
						Resources: atc.ResourceConfigs{
							{
								Name: "some-resource",
								Type: "some-type",
								Tags: atc.Tags{"val-1", "val-2"},
								Source: atc.Source{
									"private_key": `-----BEGIN SOME KEY-----
this is super secure
-----END SOME KEY-----
`,
									"config-a": "some-param-a",
									"config-b": "some-param-b-via-i",
									"bool":     true,
									"number":   1.23,
								},
							},
						},

						Jobs: atc.JobConfigs{
							{
								Name: "some-job",
								PlanSequence: []atc.Step{
									{
										Config: &atc.GetStep{
											Name: "some-resource",
										},
									},
								},
							},
						},
					})
				})

				It("succeeds", func() {
					Expect(func() {
						flyCmd := exec.Command(
							flyPath, "-t", targetName,
							"set-pipeline",
							"-n",
							"--pipeline", "awesome-pipeline",
							"-c", "fixtures/vars-pipeline.yml",
							"-l", "fixtures/vars-pipeline-params-a.yml",
							"-l", "fixtures/vars-pipeline-params-types.yml",
							"-i", "param-b=some-param-b-via-i",
						)

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())
						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(5))
				})
			})

			Context("when var flags use dot notation", func() {
				BeforeEach(func() {
					expectSaveConfig(atc.Config{
						Resources: atc.ResourceConfigs{
							{
								Name: "some-resource",
								Type: "some-type",
								Source: atc.Source{
									"a":     "foo",
									"b":     "bar",
									"other": "baz",
								},
							},
						},

						Jobs: atc.JobConfigs{
							{
								Name: "some-job",
								PlanSequence: []atc.Step{
									{
										Config: &atc.GetStep{
											Name: "some-resource",
										},
									},
								},
							},
						},
					})
				})

				It("succeeds", func() {
					Expect(func() {
						flyCmd := exec.Command(
							flyPath, "-t", targetName,
							"set-pipeline",
							"-n",
							"--pipeline", "awesome-pipeline",
							"-c", "fixtures/nested-vars-pipeline.yml",
							"-v", "source.a=foo",
							"-v", "source.b=bar",
							"-v", `"source.a"=baz`,
						)

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())
						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(5))
				})
			})

			Context("when a var is not specified", func() {
				BeforeEach(func() {
					config = atc.Config{
						Resources: atc.ResourceConfigs{
							{
								Name: "some-resource",
								Type: "some-type",
								Tags: atc.Tags{"val-1", "val-2"},
								Source: atc.Source{
									"private_key": `-----BEGIN SOME KEY-----
this is super secure
-----END SOME KEY-----
`,
									"config-a": "some-param-a",
									"config-b": "((param-b))",
									"bool":     true,
									"number":   1.23,
								},
							},
						},

						Jobs: atc.JobConfigs{
							{
								Name: "some-job",
								PlanSequence: []atc.Step{
									{
										Config: &atc.GetStep{
											Name: "some-resource",
										},
									},
								},
							},
						},
					}
					expectSaveConfig(config)
				})

				It("succeeds, sending the remaining vars uninterpolated", func() {
					Expect(func() {
						flyCmd := exec.Command(
							flyPath, "-t", targetName,
							"set-pipeline",
							"-n",
							"--pipeline", "awesome-pipeline",
							"-c", "fixtures/vars-pipeline.yml",
							"-l", "fixtures/vars-pipeline-params-a.yml",
							"-l", "fixtures/vars-pipeline-params-types.yml",
						)

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())
						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(5))
				})

				Context("when the --check-creds option is used", func() {
					Context("when the variable exists in the credentials manager", func() {
						It("should succeed and send the vars uninterpolated", func() {
							Expect(func() {
								flyCmd := exec.Command(
									flyPath, "-t", targetName,
									"set-pipeline",
									"-n",
									"--pipeline", "awesome-pipeline",
									"-c", "fixtures/vars-pipeline.yml",
									"-l", "fixtures/vars-pipeline-params-a.yml",
									"-l", "fixtures/vars-pipeline-params-types.yml",
									"--check-creds",
								)

								sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
								Expect(err).NotTo(HaveOccurred())
								<-sess.Exited
								Expect(sess.ExitCode()).To(Equal(0))
							}).To(Change(func() int {
								return len(atcServer.ReceivedRequests())
							}).By(5))
						})
					})

					Context("when the variable does not exist in the credentials manager", func() {
						BeforeEach(func() {
							path, err := atc.Routes.CreatePathForRoute(atc.SaveConfig, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
							Expect(err).NotTo(HaveOccurred())

							configResponse := atc.SaveConfigResponse{Errors: []string{"some-error"}}
							atcServer.RouteToHandler("PUT", path,
								ghttp.CombineHandlers(
									ghttp.VerifyHeaderKV(atc.ConfigVersionHeader, "42"),
									func(w http.ResponseWriter, r *http.Request) {
										bodyConfig := getConfig(r)

										receivedConfig := atc.Config{}
										err = yaml.Unmarshal(bodyConfig, &receivedConfig)
										Expect(err).NotTo(HaveOccurred())

										Expect(receivedConfig).To(Equal(config))
									},
									ghttp.RespondWithJSONEncoded(http.StatusBadRequest, configResponse, http.Header{atc.ConfigVersionHeader: {"42"}}),
								),
							)
						})

						It("should error and return the missing field", func() {
							Expect(func() {
								flyCmd := exec.Command(
									flyPath, "-t", targetName,
									"set-pipeline",
									"-n",
									"--pipeline", "awesome-pipeline",
									"-c", "fixtures/vars-pipeline.yml",
									"-l", "fixtures/vars-pipeline-params-a.yml",
									"-l", "fixtures/vars-pipeline-params-types.yml",
									"--check-creds",
								)

								sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
								Expect(err).NotTo(HaveOccurred())

								Eventually(sess.Err).Should(gbytes.Say(`error: invalid pipeline config:`))
								Eventually(sess.Err).Should(gbytes.Say(`some-error`))

								<-sess.Exited
								Expect(sess.ExitCode()).NotTo(Equal(0))
							}).To(Change(func() int {
								return len(atcServer.ReceivedRequests())
							}).By(4))
						})
					})
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

				configFile, err = os.CreateTemp("", "fly-config-file")
				Expect(err).NotTo(HaveOccurred())

				changedConfig = config

				path, err := atc.Routes.CreatePathForRoute(atc.GetConfig, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
				Expect(err).NotTo(HaveOccurred())

				atcServer.RouteToHandler("GET", path,
					ghttp.RespondWithJSONEncoded(http.StatusOK, atc.ConfigResponse{Config: config}, http.Header{atc.ConfigVersionHeader: {"42"}}),
				)

				path_get, err := atc.Routes.CreatePathForRoute(atc.GetPipeline, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
				Expect(err).NotTo(HaveOccurred())

				atcServer.RouteToHandler("GET", path_get,
					ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Pipeline{Name: "awesome-pipeline", Paused: false, TeamName: "main"}),
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
					flyCmd := exec.Command(flyPath, "-t", targetName, "set-pipeline", "-c", configFile.Name())

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))

					Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("p", "pipeline") + "' was not specified"))
				})
			})

			Context("when specifying a pipeline name with a '/' character in it", func() {
				It("fails and says '/' characters are not allowed", func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "set-pipeline", "-p", "forbidden/pipelinename", "-c", configFile.Name())

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))

					Expect(sess.Err).To(gbytes.Say("error: pipeline name cannot contain '/'"))
				})
			})

			Context("when not specifying a config file", func() {
				It("fails and says you should give a config file", func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "set-pipeline", "-p", "awesome-pipeline")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))

					Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("c", "config") + "' was not specified"))
				})
			})

			Context("when configuring with groups re-ordered", func() {
				BeforeEach(func() {
					changedConfig.Groups = atc.GroupConfigs{
						{
							Name:      "some-other-group",
							Jobs:      []string{"job-3", "job-4"},
							Resources: []string{"resource-6", "resource-4"},
						},
						{
							Name:      "some-group",
							Jobs:      []string{"job-1", "job-2"},
							Resources: []string{"resource-1", "resource-2"},
						},
					}

					path, err := atc.Routes.CreatePathForRoute(atc.SaveConfig, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
					Expect(err).NotTo(HaveOccurred())

					atcServer.RouteToHandler("PUT", path,
						ghttp.CombineHandlers(
							ghttp.VerifyHeaderKV(atc.ConfigVersionHeader, "42"),
							func(w http.ResponseWriter, r *http.Request) {
								w.Header().Set("Content-Type", "application/json")
								config := getConfig(r)
								Expect(config).To(MatchYAML(payload))
							},
							ghttp.RespondWith(http.StatusOK, "{}"),
						),
					)
				})

				It("parses the config file and sends it to the ATC", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "set-pipeline", "-p", "awesome-pipeline", "-c", configFile.Name())

						stdin, err := flyCmd.StdinPipe()
						Expect(err).NotTo(HaveOccurred())

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess).Should(gbytes.Say("group some-group has changed"))

						Eventually(sess).Should(gbytes.Say("group some-other-group has changed"))

						Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
						yes(stdin)

						Eventually(sess).Should(gbytes.Say("configuration updated"))

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))

					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(5))
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

					newResources := make(atc.ResourceConfigs, len(changedConfig.Resources))
					copy(newResources, changedConfig.Resources)
					newResources[0].Type = "some-new-type"
					newResources[1] = newResource
					newResources[2].Source = atc.Source{"source-config": 5.0}

					changedConfig.Resources = newResources

					newResourceType := changedConfig.ResourceTypes[1]
					newResourceType.Name = "some-new-resource-type"

					newResourceTypes := make(atc.ResourceTypes, len(changedConfig.ResourceTypes))
					copy(newResourceTypes, changedConfig.ResourceTypes)
					newResourceTypes[0].Type = "some-new-type"
					newResourceTypes[1] = newResourceType

					changedConfig.ResourceTypes = newResourceTypes

					newJob := changedConfig.Jobs[2]
					newJob.Name = "some-new-job"
					changedConfig.Jobs[0].Serial = false
					changedConfig.Jobs = append(changedConfig.Jobs[:2], newJob)

					path, err := atc.Routes.CreatePathForRoute(atc.SaveConfig, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
					Expect(err).NotTo(HaveOccurred())

					atcServer.RouteToHandler("PUT", path,
						ghttp.CombineHandlers(
							ghttp.VerifyHeaderKV(atc.ConfigVersionHeader, "42"),
							func(w http.ResponseWriter, r *http.Request) {
								w.Header().Set("Content-Type", "application/json")
								config := getConfig(r)
								Expect(config).To(MatchYAML(payload))
							},
							ghttp.RespondWith(http.StatusOK, "{}"),
						),
					)
				})

				It("parses the config file and sends it to the ATC", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "set-pipeline", "-p", "awesome-pipeline", "-c", configFile.Name())

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

						Eventually(sess).Should(gbytes.Say("resource type some-resource-type has changed"))
						Eventually(sess.Out.Contents).Should(ContainSubstring(ansi.Color("type: some-type", "red")))
						Eventually(sess.Out.Contents).Should(ContainSubstring(ansi.Color("type: some-new-type", "green")))

						Eventually(sess).Should(gbytes.Say("resource type some-other-resource-type has been removed"))
						Eventually(sess.Out.Contents).Should(ContainSubstring(ansi.Color("name: some-other-resource-type", "red")))

						Eventually(sess).Should(gbytes.Say("resource type some-new-resource-type has been added"))
						Eventually(sess.Out.Contents).Should(ContainSubstring(ansi.Color("name: some-new-resource-type", "green")))

						Eventually(sess).Should(gbytes.Say("job some-job has changed"))
						Eventually(sess.Out.Contents).Should(ContainSubstring(ansi.Color("serial: true", "red")))

						Eventually(sess).Should(gbytes.Say("job some-other-job has been removed"))
						Eventually(sess.Out.Contents).Should(ContainSubstring(ansi.Color("name: some-other-job", "red")))

						Eventually(sess).Should(gbytes.Say("job some-new-job has been added"))
						Eventually(sess.Out.Contents).Should(ContainSubstring(ansi.Color("name: some-new-job", "green")))

						Eventually(sess).Should(gbytes.Say("pipeline name:"))
						Eventually(sess).Should(gbytes.Say("awesome-pipeline"))
						Consistently(sess).ShouldNot(gbytes.Say("pipeline instance vars:"))

						Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
						yes(stdin)

						Eventually(sess).Should(gbytes.Say("configuration updated"))

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))

						Expect(sess.Out.Contents()).ToNot(ContainSubstring("some-resource-with-int-field"))

						Expect(sess.Out.Contents()).ToNot(ContainSubstring("some-unchanged-job"))

					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(5))
				})

				It("bails if the user rejects the diff", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "set-pipeline", "-p", "awesome-pipeline", "-c", configFile.Name())

						stdin, err := flyCmd.StdinPipe()
						Expect(err).NotTo(HaveOccurred())

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
						no(stdin)

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(3))
				})

				It("parses the config from stdin and sends it to the ATC", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "set-pipeline", "-p", "awesome-pipeline", "-c", "-")

						stdin, err := flyCmd.StdinPipe()
						Expect(err).NotTo(HaveOccurred())

						file, err := os.Open(configFile.Name())
						Expect(err).NotTo(HaveOccurred())
						_, err = io.Copy(stdin, file)
						Expect(err).NotTo(HaveOccurred())
						file.Close()
						stdin.Close()

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

						Eventually(sess).Should(gbytes.Say("resource type some-resource-type has changed"))
						Eventually(sess.Out.Contents).Should(ContainSubstring(ansi.Color("type: some-type", "red")))
						Eventually(sess.Out.Contents).Should(ContainSubstring(ansi.Color("type: some-new-type", "green")))

						Eventually(sess).Should(gbytes.Say("resource type some-other-resource-type has been removed"))
						Eventually(sess.Out.Contents).Should(ContainSubstring(ansi.Color("name: some-other-resource-type", "red")))

						Eventually(sess).Should(gbytes.Say("resource type some-new-resource-type has been added"))
						Eventually(sess.Out.Contents).Should(ContainSubstring(ansi.Color("name: some-new-resource-type", "green")))

						Eventually(sess).Should(gbytes.Say("job some-job has changed"))
						Eventually(sess.Out.Contents).Should(ContainSubstring(ansi.Color("serial: true", "red")))

						Eventually(sess).Should(gbytes.Say("job some-other-job has been removed"))
						Eventually(sess.Out.Contents).Should(ContainSubstring(ansi.Color("name: some-other-job", "red")))

						Eventually(sess).Should(gbytes.Say("job some-new-job has been added"))
						Eventually(sess.Out.Contents).Should(ContainSubstring(ansi.Color("name: some-new-job", "green")))

						// When read pipeline configure from stdin, it should do non-interactive mode.
						Consistently(sess).ShouldNot(gbytes.Say(`apply configuration\? \[yN\]: `))

						Eventually(sess).Should(gbytes.Say("configuration updated"))

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))

						Expect(sess.Out.Contents()).ToNot(ContainSubstring("some-resource-with-int-field"))

						Expect(sess.Out.Contents()).ToNot(ContainSubstring("some-unchanged-job"))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(5))
				})

				Context("when setting an instanced pipeline", func() {
					It("prints the instance vars as YAML", func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "set-pipeline", "-p", "awesome-pipeline", "-i", "version=1.2.3", "-c", configFile.Name())

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess).Should(gbytes.Say("pipeline name:"))
						Eventually(sess).Should(gbytes.Say("awesome-pipeline"))

						Eventually(sess).Should(gbytes.Say("pipeline instance vars:"))
						Eventually(sess).Should(gbytes.Say("  version: 1.2.3"))
					})
				})
			})

			Context("when setting new pipeline with non-default team", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/teams/other-team"),
							ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Team{
								Name: "other-team",
							}),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/teams/other-team/pipelines/awesome-pipeline/config"),
							ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Team{
								Name: "other-team",
							}),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/teams/other-team/pipelines/awesome-pipeline"),
							ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Pipeline{
								Name: "awesome-pipeline",
							}),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", "/api/v1/teams/other-team/pipelines/awesome-pipeline/config"),
							ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Team{
								Name: "other-team",
							}),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/teams/other-team/pipelines/awesome-pipeline"),
							ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Pipeline{
								Name: "awesome-pipeline",
							}),
						),
					)
				})

				It("successfully sets new pipeline to non-default team", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "set-pipeline", "-p", "awesome-pipeline", "-c", configFile.Name(), "--team", "other-team")

						stdin, err := flyCmd.StdinPipe()
						Expect(err).NotTo(HaveOccurred())

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
						yes(stdin)

						Eventually(sess).Should(gbytes.Say("configuration updated"))

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))

					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(6))
				})

				It("bails if the user rejects the configuration", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "set-pipeline", "-p", "awesome-pipeline", "-c", configFile.Name(), "--team", "other-team")

						stdin, err := flyCmd.StdinPipe()
						Expect(err).NotTo(HaveOccurred())

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
						no(stdin)
						Eventually(sess).Should(gbytes.Say("bailing out"))

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(4))
				})
			})

			Context("when configuring fails", func() {
				BeforeEach(func() {
					path, err := atc.Routes.CreatePathForRoute(atc.SaveConfig, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
					Expect(err).NotTo(HaveOccurred())

					atcServer.RouteToHandler("PUT", path,
						ghttp.RespondWith(http.StatusInternalServerError, "nope"),
					)
					config.Resources[0].Name = "updated-name"
				})

				It("prints the error to stderr and exits 1", func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "set-pipeline", "-c", configFile.Name(), "-p", "awesome-pipeline")

					stdin, err := flyCmd.StdinPipe()
					Expect(err).NotTo(HaveOccurred())

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
					yes(stdin)

					Eventually(sess.Err).Should(gbytes.Say("500 Internal Server Error"))
					Eventually(sess.Err).Should(gbytes.Say("nope"))

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))
				})
			})

			Context("when updating a pipeline that has been configured through the 'set_pipeline' step", func() {
				BeforeEach(func() {
					path, err := atc.Routes.CreatePathForRoute(atc.SaveConfig, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
					Expect(err).NotTo(HaveOccurred())

					path_get, err := atc.Routes.CreatePathForRoute(atc.GetPipeline, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
					Expect(err).NotTo(HaveOccurred())

					atcServer.RouteToHandler("PUT", path, ghttp.CombineHandlers(
						ghttp.VerifyHeaderKV(atc.ConfigVersionHeader, "42"),
						func(w http.ResponseWriter, r *http.Request) {
							w.Header().Set("Content-Type", "application/json")
							config := getConfig(r)
							Expect(config).To(MatchYAML(payload))
						},
						ghttp.RespondWith(http.StatusCreated, "{}"),
					))

					atcServer.RouteToHandler("GET", path_get, ghttp.RespondWithJSONEncoded(http.StatusOK,
						atc.Pipeline{ID: 1, Name: "awesome-pipeline", Paused: false, TeamName: "main", ParentBuildID: 321, ParentJobID: 123}))

					config.Jobs[0].Name = "updated-name"
				})

				It("succeeds and prints a warning message", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "set-pipeline", "-p", "awesome-pipeline", "-c", configFile.Name())

						stdin, err := flyCmd.StdinPipe()
						Expect(err).NotTo(HaveOccurred())

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess).Should(gbytes.Say("WARNING: pipeline has been configured through the 'set_pipeline' step, your changes may be overwritten on the next 'set_pipeline' step execution"))

						Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
						yes(stdin)

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(5))
				})
			})

			Context("when dry-run mode has been enabled whilst setting a pipeline", func() {
				BeforeEach(func() {
					path, err := atc.Routes.CreatePathForRoute(atc.SaveConfig, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
					Expect(err).NotTo(HaveOccurred())

					path_get, err := atc.Routes.CreatePathForRoute(atc.GetPipeline, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
					Expect(err).NotTo(HaveOccurred())

					atcServer.RouteToHandler("PUT", path, ghttp.CombineHandlers(
						ghttp.VerifyHeaderKV(atc.ConfigVersionHeader, "42"),
						func(w http.ResponseWriter, r *http.Request) {
							config := getConfig(r)
							Expect(config).To(MatchYAML(payload))
						},
						ghttp.RespondWith(http.StatusCreated, "{}"),
					))

					atcServer.RouteToHandler("GET", path_get, ghttp.RespondWithJSONEncoded(http.StatusOK,
						atc.Pipeline{ID: 1, Name: "awesome-pipeline", Paused: false, TeamName: "main", ParentBuildID: 321, ParentJobID: 123}))

					config.Jobs[0].Name = "updated-name"
				})

				It("prints pipeline diff and exits", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "set-pipeline", "-p", "awesome-pipeline", "-c", configFile.Name(), "-d")

						_, err := flyCmd.StdinPipe()
						Expect(err).NotTo(HaveOccurred())

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess).Should(gbytes.Say("Dry-run mode was set, exiting."))

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(2))
				})
			})

			Context("when the pipeline is paused", func() {
				AssertSuccessWithPausedPipelineHelp := func(expectCreationMessage bool) {
					It("succeeds and prints a message to help the user", func() {
						Expect(func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "set-pipeline", "-p", "awesome-pipeline", "-c", configFile.Name())

							stdin, err := flyCmd.StdinPipe()
							Expect(err).NotTo(HaveOccurred())

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
							yes(stdin)

							if expectCreationMessage {
								pipelineURL, err := url.JoinPath(atcServer.URL(), "/teams/main/pipelines/awesome-pipeline")
								Expect(err).NotTo(HaveOccurred())

								Eventually(sess).Should(gbytes.Say("pipeline created!"))
								Eventually(sess).Should(gbytes.Say(fmt.Sprintf("you can view your pipeline here: %s", pipelineURL)))
							}

							Eventually(sess).Should(gbytes.Say("the pipeline is currently paused. to unpause, either:"))
							Eventually(sess).Should(gbytes.Say("  - run the unpause-pipeline command:"))
							Eventually(sess).Should(gbytes.Say("    %s -t %s unpause-pipeline -p awesome-pipeline", regexp.QuoteMeta(flyPath), targetName))
							Eventually(sess).Should(gbytes.Say("  - click play next to the pipeline in the web ui"))

							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(0))
						}).To(Change(func() int {
							return len(atcServer.ReceivedRequests())
						}).By(5))
					})
				}

				Context("when updating an existing pipeline", func() {
					BeforeEach(func() {
						path, err := atc.Routes.CreatePathForRoute(atc.SaveConfig, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
						Expect(err).NotTo(HaveOccurred())

						path_get, err := atc.Routes.CreatePathForRoute(atc.GetPipeline, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
						Expect(err).NotTo(HaveOccurred())

						atcServer.RouteToHandler("PUT", path, ghttp.CombineHandlers(
							ghttp.VerifyHeaderKV(atc.ConfigVersionHeader, "42"),
							func(w http.ResponseWriter, r *http.Request) {
								config := getConfig(r)
								w.Header().Set("Content-Type", "application/json")
								Expect(config).To(MatchYAML(payload))
							},

							ghttp.RespondWith(http.StatusOK, "{}"),
						))

						atcServer.RouteToHandler("GET", path_get, ghttp.RespondWithJSONEncoded(http.StatusOK,
							atc.Pipeline{ID: 1, Name: "awesome-pipeline", Paused: true, TeamName: "main"}))

						config.Resources[0].Name = "updated-name"
					})

					AssertSuccessWithPausedPipelineHelp(false)
				})

				Context("when the pipeline is being created for the first time", func() {
					BeforeEach(func() {
						path, err := atc.Routes.CreatePathForRoute(atc.SaveConfig, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
						Expect(err).NotTo(HaveOccurred())

						path_get, err := atc.Routes.CreatePathForRoute(atc.GetPipeline, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
						Expect(err).NotTo(HaveOccurred())

						atcServer.RouteToHandler("PUT", path, ghttp.CombineHandlers(
							ghttp.VerifyHeaderKV(atc.ConfigVersionHeader, "42"),
							func(w http.ResponseWriter, r *http.Request) {
								config := getConfig(r)
								w.Header().Set("Content-Type", "application/json")
								Expect(config).To(MatchYAML(payload))
							},
							ghttp.RespondWith(http.StatusCreated, "{}"),
						))

						atcServer.RouteToHandler("GET", path_get, ghttp.RespondWithJSONEncoded(http.StatusOK,
							atc.Pipeline{ID: 1, Name: "awesome-pipeline", Paused: true, TeamName: "main"}))

						config.Resources[0].Name = "updated-name"
					})

					AssertSuccessWithPausedPipelineHelp(true)
				})
			})

			Context("when the server returns warnings", func() {
				BeforeEach(func() {
					path, err := atc.Routes.CreatePathForRoute(atc.SaveConfig, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
					Expect(err).NotTo(HaveOccurred())

					atcServer.RouteToHandler("PUT", path, ghttp.CombineHandlers(
						ghttp.VerifyHeaderKV(atc.ConfigVersionHeader, "42"),
						func(w http.ResponseWriter, r *http.Request) {
							config := getConfig(r)
							w.Header().Set("Content-Type", "application/json")
							Expect(config).To(MatchYAML(payload))
						},
						ghttp.RespondWith(http.StatusCreated, `{"warnings":[
							{"type":"deprecation","message":"warning-1"},
							{"type":"deprecation","message":"warning-2"}
						]}`),
					))
					config.Resources[0].Name = "updated-name"
				})

				It("succeeds and prints warnings", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "set-pipeline", "-p", "awesome-pipeline", "-c", configFile.Name())

						stdin, err := flyCmd.StdinPipe()
						Expect(err).NotTo(HaveOccurred())

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
						yes(stdin)

						Eventually(sess.Err).Should(gbytes.Say("DEPRECATION WARNING:"))
						Eventually(sess.Err).Should(gbytes.Say("  - warning-1"))
						Eventually(sess.Err).Should(gbytes.Say("  - warning-2"))
						Eventually(sess).Should(gbytes.Say("pipeline created!"))

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(5))
				})
			})

			Context("when there are no pipeline changes", func() {
				It("does not ask for user interaction to apply changes", func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "set-pipeline", "-p", "awesome-pipeline", "-c", configFile.Name())

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Consistently(sess).ShouldNot(gbytes.Say("pipeline name:"))
					Consistently(sess).ShouldNot(gbytes.Say(`apply configuration\? \[yN\]: `))

					Eventually(sess).Should(gbytes.Say("no changes to apply"))

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))

				})
			})

			Context("when the server rejects the request", func() {
				BeforeEach(func() {
					path, err := atc.Routes.CreatePathForRoute(atc.SaveConfig, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
					Expect(err).NotTo(HaveOccurred())

					atcServer.RouteToHandler("PUT", path, func(w http.ResponseWriter, r *http.Request) {
						atcServer.CloseClientConnections()
					})
					config.Resources[0].Name = "updated-name"
				})

				It("prints the error to stderr and exits 1", func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "set-pipeline", "-c", configFile.Name(), "-p", "awesome-pipeline")

					stdin, err := flyCmd.StdinPipe()
					Expect(err).NotTo(HaveOccurred())

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
					yes(stdin)

					Eventually(sess.Err).Should(gbytes.Say("EOF"))

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))
				})
			})
		})
	})
})

func getConfig(r *http.Request) []byte {
	defer r.Body.Close()
	payload, err := io.ReadAll(r.Body)
	Expect(err).NotTo(HaveOccurred())

	return payload
}
