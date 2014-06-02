package builder_test

import (
	"net/http"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/router"

	ProleBuilds "github.com/winston-ci/prole/api/builds"
	ProleRoutes "github.com/winston-ci/prole/routes"

	WinstonRoutes "github.com/winston-ci/winston/api/routes"
	. "github.com/winston-ci/winston/builder"
	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/config"
	"github.com/winston-ci/winston/db"
	"github.com/winston-ci/winston/redisrunner"
)

var _ = Describe("Builder", func() {
	var redisRunner *redisrunner.Runner
	var redis db.DB

	var proleServer *ghttp.Server

	var builder Builder

	var job config.Job
	var resources config.Resources

	BeforeEach(func() {
		redisRunner = redisrunner.NewRunner()
		redisRunner.Start()

		redis = db.NewRedis(redisRunner.Pool())

		proleServer = ghttp.NewServer()

		job = config.Job{
			Name: "foo",

			Privileged: true,

			BuildConfigPath: "some-resource/build.yml",

			Inputs: []config.Input{
				{
					Resource: "some-resource",
				},
			},
		}

		resources = config.Resources{
			{
				Name:   "some-resource",
				Type:   "git",
				Source: config.Source{"uri": "git://some-resource"},
			},
		}
	})

	JustBeforeEach(func() {
		builder = NewBuilder(
			redis,
			resources,
			router.NewRequestGenerator(proleServer.URL(), ProleRoutes.Routes),
			router.NewRequestGenerator("http://winston-server", WinstonRoutes.Routes),
		)
	})

	AfterEach(func() {
		redisRunner.Stop()
	})

	Describe("Creating a build", func() {
		It("triggers a build on the prole endpoint", func() {
			proleServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/builds"),
					ghttp.VerifyJSONRepresenting(ProleBuilds.Build{
						Privileged: true,

						Callback: "http://winston-server/builds/foo/1",
						LogsURL:  "ws://winston-server/builds/foo/1/log/input",

						Inputs: []ProleBuilds.Input{
							{
								Name:            "some-resource",
								Type:            "git",
								Source:          ProleBuilds.Source{"uri": "git://some-resource"},
								DestinationPath: "some-resource",
								ConfigPath:      "build.yml",
							},
						},

						Outputs: []ProleBuilds.Output{},
					}),
					ghttp.RespondWith(201, ""),
				),
			)

			build, err := builder.Create(job, nil)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(build.ID).Should(Equal(1))
		})

		It("returns increasing build numbers", func() {
			proleServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/builds"),
					ghttp.RespondWith(201, ""),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/builds"),
					ghttp.RespondWith(201, ""),
				),
			)

			build, err := builder.Create(job, nil)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(build.ID).Should(Equal(1))

			build, err = builder.Create(job, nil)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(build.ID).Should(Equal(2))
		})

		Context("when the job is serial", func() {
			BeforeEach(func() {
				job.Serial = true
			})

			Context("and the current build is started", func() {
				var existingBuild builds.Build

				BeforeEach(func() {
					var err error

					existingBuild, err = redis.CreateBuild(job.Name)
					Ω(err).ShouldNot(HaveOccurred())

					err = redis.SaveBuildStatus(job.Name, existingBuild.ID, builds.StatusStarted)
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("returns a pending build", func() {
					queuedBuild, err := builder.Create(job, nil)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(queuedBuild.ID).Should(Equal(existingBuild.ID + 1))
					Ω(queuedBuild.Status).Should(Equal(builds.StatusPending))
				})

				It("does not trigger a build", func() {
					_, err := builder.Create(job, nil)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(proleServer.ReceivedRequests()).Should(BeEmpty())
				})
			})

			for _, s := range []builds.Status{builds.StatusSucceeded, builds.StatusFailed, builds.StatusErrored} {
				status := s

				Context("and the current build is "+string(status), func() {
					var existingBuild builds.Build

					BeforeEach(func() {
						var err error

						existingBuild, err = redis.CreateBuild(job.Name)
						Ω(err).ShouldNot(HaveOccurred())

						existingBuild, err = redis.StartBuild(job.Name, existingBuild.ID, true)
						Ω(err).ShouldNot(HaveOccurred())

						err = redis.SaveBuildStatus(job.Name, existingBuild.ID, status)
						Ω(err).ShouldNot(HaveOccurred())
					})

					It("triggers a build", func() {
						proleServer.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("POST", "/builds"),
								ghttp.RespondWith(201, ""),
							),
						)

						queuedBuild, err := builder.Create(job, nil)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(queuedBuild.ID).Should(Equal(existingBuild.ID + 1))
						Ω(queuedBuild.Status).Should(Equal(builds.StatusStarted))
					})
				})
			}
		})

		Context("when the build has outputs", func() {
			BeforeEach(func() {
				job.Outputs = []config.Output{
					{
						Resource: "some-resource",
						Params:   config.Params{"foo": "bar"},
					},
				}
			})

			It("sends them along to the prole", func() {
				proleServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/builds"),
						ghttp.VerifyJSONRepresenting(ProleBuilds.Build{
							Privileged: true,

							Callback: "http://winston-server/builds/foo/1",
							LogsURL:  "ws://winston-server/builds/foo/1/log/input",

							Inputs: []ProleBuilds.Input{
								{
									Name:            "some-resource",
									Type:            "git",
									Source:          ProleBuilds.Source{"uri": "git://some-resource"},
									DestinationPath: "some-resource",
									ConfigPath:      "build.yml",
								},
							},

							Outputs: []ProleBuilds.Output{
								{
									Name:       "some-resource",
									Type:       "git",
									Params:     ProleBuilds.Params{"foo": "bar"},
									SourcePath: "some-resource",
								},
							},
						}),
						ghttp.RespondWith(201, ""),
					),
				)

				_, err := builder.Create(job, nil)
				Ω(err).ShouldNot(HaveOccurred())
			})

			Context("and one of the outputs is not specified as an input", func() {
				BeforeEach(func() {
					resources = append(resources, config.Resource{
						Name:   "some-output-resource",
						Type:   "git",
						Source: config.Source{"uri": "git://some-output-resource"},
					})

					job.Outputs = append(job.Outputs, config.Output{
						Resource: "some-output-resource",
						Params:   config.Params{"fizz": "buzz"},
					})
				})

				It("is specified as an input", func() {
					proleServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/builds"),
							ghttp.VerifyJSONRepresenting(ProleBuilds.Build{
								Privileged: true,

								Callback: "http://winston-server/builds/foo/1",
								LogsURL:  "ws://winston-server/builds/foo/1/log/input",

								Inputs: []ProleBuilds.Input{
									{
										Name:            "some-resource",
										Type:            "git",
										Source:          ProleBuilds.Source{"uri": "git://some-resource"},
										DestinationPath: "some-resource",
										ConfigPath:      "build.yml",
									},
									{
										Name:            "some-output-resource",
										Type:            "git",
										Source:          ProleBuilds.Source{"uri": "git://some-output-resource"},
										DestinationPath: "some-output-resource",
									},
								},

								Outputs: []ProleBuilds.Output{
									{
										Name:       "some-resource",
										Type:       "git",
										Params:     ProleBuilds.Params{"foo": "bar"},
										SourcePath: "some-resource",
									},
									{
										Name:       "some-output-resource",
										Type:       "git",
										Params:     ProleBuilds.Params{"fizz": "buzz"},
										SourcePath: "some-output-resource",
									},
								},
							}),
							ghttp.RespondWith(201, ""),
						),
					)

					_, err := builder.Create(job, nil)
					Ω(err).ShouldNot(HaveOccurred())
				})
			})
		})

		Context("when resource versions are specified", func() {
			It("uses them for the build's inputs", func() {
				proleServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/builds"),
						ghttp.VerifyJSONRepresenting(ProleBuilds.Build{
							Privileged: true,

							Callback: "http://winston-server/builds/foo/1",
							LogsURL:  "ws://winston-server/builds/foo/1/log/input",

							Inputs: []ProleBuilds.Input{
								{
									Name:            "some-resource",
									Type:            "git",
									Source:          ProleBuilds.Source{"uri": "git://some-resource"},
									Version:         ProleBuilds.Version{"version": "1"},
									DestinationPath: "some-resource",
									ConfigPath:      "build.yml",
								},
							},

							Outputs: []ProleBuilds.Output{},
						}),
						ghttp.RespondWith(201, ""),
					),
				)

				_, err := builder.Create(job, map[string]builds.Version{
					"some-resource": builds.Version{"version": "1"},
				})
				Ω(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when the job has a resource that depends on other jobs", func() {
			BeforeEach(func() {
				resources = append(resources, config.Resource{
					Name:   "some-dependant-resource",
					Type:   "git",
					Source: config.Source{"uri": "git://some-dependant-resource"},
				})

				job.Inputs = append(job.Inputs, config.Input{
					Resource: "some-dependant-resource",
					Passed:   []string{"job1", "job2"},
				})
			})

			Context("and the other jobs satisfy the dependency", func() {
				BeforeEach(func() {
					err := redis.SaveOutputVersion("job1", 1, "some-dependant-resource", builds.Version{"version": "1"})
					Ω(err).ShouldNot(HaveOccurred())

					err = redis.SaveOutputVersion("job2", 1, "some-dependant-resource", builds.Version{"version": "1"})
					Ω(err).ShouldNot(HaveOccurred())

					err = redis.SaveOutputVersion("job1", 1, "some-dependant-resource", builds.Version{"version": "2"})
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("builds with a source that satisfies the dependency", func() {
					proleServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/builds"),
							ghttp.VerifyJSONRepresenting(ProleBuilds.Build{
								Privileged: true,

								Callback: "http://winston-server/builds/foo/1",
								LogsURL:  "ws://winston-server/builds/foo/1/log/input",

								Inputs: []ProleBuilds.Input{
									{
										Name:            "some-resource",
										Type:            "git",
										Source:          ProleBuilds.Source{"uri": "git://some-resource"},
										DestinationPath: "some-resource",
										ConfigPath:      "build.yml",
									},
									{
										Name:            "some-dependant-resource",
										Type:            "git",
										Source:          ProleBuilds.Source{"uri": "git://some-dependant-resource"},
										Version:         ProleBuilds.Version{"version": "1"},
										DestinationPath: "some-dependant-resource",
									},
								},

								Outputs: []ProleBuilds.Output{},
							}),
							ghttp.RespondWith(201, ""),
						),
					)

					build, err := builder.Create(job, nil)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(build.ID).Should(Equal(1))
				})
			})

			Context("and the other jobs do not satisfy the dependency", func() {
				It("returns an error", func() {
					_, err := builder.Create(job, nil)
					Ω(err).Should(HaveOccurred())
				})
			})
		})

		Context("when the job's input is not found", func() {
			BeforeEach(func() {
				job.Inputs = append(job.Inputs, config.Input{
					Resource: "some-bogus-resource",
				})
			})

			It("returns an error", func() {
				_, err := builder.Create(job, nil)
				Ω(err).Should(HaveOccurred())
			})
		})

		Context("when the job's output is not found", func() {
			BeforeEach(func() {
				job.Outputs = append(job.Outputs, config.Output{
					Resource: "some-bogus-resource",
				})
			})

			It("returns an error", func() {
				_, err := builder.Create(job, nil)
				Ω(err).Should(HaveOccurred())
			})
		})

		Context("when the prole server is unreachable", func() {
			BeforeEach(func() {
				proleServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/builds"),
						func(w http.ResponseWriter, r *http.Request) {
							proleServer.HTTPTestServer.CloseClientConnections()
						},
					),
				)
			})

			It("returns an error", func() {
				_, err := builder.Create(job, nil)
				Ω(err).Should(HaveOccurred())

				Ω(proleServer.ReceivedRequests()).Should(HaveLen(1))
			})
		})

		Context("when the prole server returns non-201", func() {
			BeforeEach(func() {
				proleServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/builds"),
						ghttp.RespondWith(400, ""),
					),
				)
			})

			It("returns an error", func() {
				_, err := builder.Create(job, nil)
				Ω(err).Should(HaveOccurred())
			})
		})
	})
})
