package builder_test

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/router"

	TurbineBuilds "github.com/concourse/turbine/api/builds"
	TurbineRoutes "github.com/concourse/turbine/routes"

	WinstonRoutes "github.com/concourse/atc/api/routes"
	. "github.com/concourse/atc/builder"
	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/redisrunner"
)

var _ = Describe("Builder", func() {
	var redisRunner *redisrunner.Runner
	var redis db.DB

	var turbineServer *ghttp.Server

	var builder Builder

	var job config.Job
	var resources config.Resources

	var expectedTurbineBuild TurbineBuilds.Build

	BeforeEach(func() {
		redisRunner = redisrunner.NewRunner()
		redisRunner.Start()

		redis = db.NewRedis(redisRunner.Pool())

		turbineServer = ghttp.NewServer()

		job = config.Job{
			Name: "foo",

			BuildConfig: TurbineBuilds.Config{
				Image: "some-image",
				Params: map[string]string{
					"FOO": "1",
					"BAR": "2",
				},
				Run: TurbineBuilds.RunConfig{
					Path: "some-script",
					Args: []string{"arg1", "arg2"},
				},
			},

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
			{
				Name:   "some-dependant-resource",
				Type:   "git",
				Source: config.Source{"uri": "git://some-dependant-resource"},
			},
			{
				Name:   "some-output-resource",
				Type:   "git",
				Source: config.Source{"uri": "git://some-output-resource"},
			},
		}

		expectedTurbineBuild = TurbineBuilds.Build{
			Config: TurbineBuilds.Config{
				Image: "some-image",
				Params: map[string]string{
					"FOO": "1",
					"BAR": "2",
				},
				Run: TurbineBuilds.RunConfig{
					Path: "some-script",
					Args: []string{"arg1", "arg2"},
				},
			},

			Privileged: true,

			Callback: "http://atc-server/builds/foo/1",
			LogsURL:  "ws://atc-server/builds/foo/1/log/input",

			Inputs: []TurbineBuilds.Input{
				{
					Name:            "some-resource",
					Type:            "git",
					Source:          TurbineBuilds.Source{"uri": "git://some-resource"},
					DestinationPath: "some-resource",
					ConfigPath:      "build.yml",
				},
			},

			Outputs: []TurbineBuilds.Output{},
		}

		builder = NewBuilder(
			redis,
			resources,
			router.NewRequestGenerator(turbineServer.URL(), TurbineRoutes.Routes),
			router.NewRequestGenerator("http://atc-server", WinstonRoutes.Routes),
		)
	})

	AfterEach(func() {
		redisRunner.Stop()
	})

	successfulBuildStart := func(build TurbineBuilds.Build) http.HandlerFunc {
		createdBuild := build
		createdBuild.Guid = "some-turbine-guid"
		createdBuild.AbortURL = turbineServer.URL() + "/abort/the/build"

		return ghttp.CombineHandlers(
			ghttp.VerifyJSONRepresenting(build),
			ghttp.RespondWithJSONEncoded(201, createdBuild),
		)
	}

	Describe("Create", func() {
		It("creates a pending build", func() {
			build, err := builder.Create(job)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(build.ID).Should(Equal(1))
		})

		It("returns increasing build numbers", func() {
			build, err := builder.Create(job)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(build.ID).Should(Equal(1))

			build, err = builder.Create(job)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(build.ID).Should(Equal(2))
		})
	})

	Describe("Attempt", func() {
		// the full behavior of these would be duped in the db tests, so only
		// a small piece is covered

		resource := config.Resource{Name: "foo"}
		version := builds.Version{"version": "2"}

		It("attempts to create a pending build", func() {
			build, err := builder.Attempt(job, resource, version)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(build.ID).Should(Equal(1))
		})

		It("cannot be done concurrently", func() {
			build, err := builder.Attempt(job, resource, version)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(build.ID).Should(Equal(1))

			_, err = builder.Attempt(job, resource, version)
			Ω(err).Should(HaveOccurred())
		})

		Context("when the resource is in the job's inputs and outputs", func() {
			BeforeEach(func() {
				job.Inputs = append(job.Inputs, config.Input{
					Resource: "foo",
				})

				job.Outputs = append(job.Outputs, config.Output{
					Resource: "foo",
				})
			})

			Context("and a build is running", func() {
				BeforeEach(func() {
					runningBuild, err := redis.CreateBuild(job.Name)
					Ω(err).ShouldNot(HaveOccurred())

					scheduled, err := redis.ScheduleBuild(job.Name, runningBuild.ID, true)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeTrue())

					err = redis.SaveBuildInput(job.Name, runningBuild.ID, builds.Input{
						Name:    "foo",
						Version: builds.Version{"version": "1"},
					})
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("fails", func() {
					_, err := builder.Attempt(job, resource, version)
					Ω(err).Should(HaveOccurred())
				})
			})
		})
	})

	Describe("Starting a build", func() {
		var build builds.Build

		BeforeEach(func() {
			var err error

			build, err = builder.Create(job)
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("triggers a build on the turbine endpoint", func() {
			turbineServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/builds"),
					successfulBuildStart(expectedTurbineBuild),
				),
			)

			build, err := builder.Start(job, build, nil)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(build.ID).Should(Equal(1))
		})

		Context("when the job is serial", func() {
			BeforeEach(func() {
				job.Serial = true
			})

			Context("and the current build is scheduled", func() {
				var existingBuild builds.Build

				BeforeEach(func() {
					var err error

					existingBuild, err = redis.CreateBuild(job.Name)
					Ω(err).ShouldNot(HaveOccurred())

					scheduled, err := redis.ScheduleBuild(job.Name, existingBuild.ID, false)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeTrue())
				})

				It("returns the build, still pending", func() {
					queuedBuild, err := builder.Start(job, build, nil)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(queuedBuild.Status).Should(Equal(builds.StatusPending))
				})

				It("does not trigger a build", func() {
					_, err := builder.Start(job, build, nil)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(turbineServer.ReceivedRequests()).Should(BeEmpty())
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

						scheduled, err := redis.ScheduleBuild(job.Name, existingBuild.ID, false)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeTrue())

						err = redis.SaveBuildStatus(job.Name, existingBuild.ID, status)
						Ω(err).ShouldNot(HaveOccurred())
					})

					It("starts the build", func() {
						turbineServer.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("POST", "/builds"),
								successfulBuildStart(expectedTurbineBuild),
							),
						)

						queuedBuild, err := builder.Start(job, build, nil)
						Ω(err).ShouldNot(HaveOccurred())

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

				expectedTurbineBuild.Outputs = []TurbineBuilds.Output{
					{
						Name:       "some-resource",
						Type:       "git",
						Params:     TurbineBuilds.Params{"foo": "bar"},
						SourcePath: "some-resource",
						Source:     TurbineBuilds.Source{"uri": "git://some-resource"},
					},
				}
			})

			It("sends them along to the turbine", func() {

				turbineServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/builds"),
						successfulBuildStart(expectedTurbineBuild),
					),
				)

				_, err := builder.Start(job, build, nil)
				Ω(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when resource versions are specified", func() {
			BeforeEach(func() {
				expectedTurbineBuild.Inputs = []TurbineBuilds.Input{
					{
						Name:            "some-resource",
						Type:            "git",
						Source:          TurbineBuilds.Source{"uri": "git://some-resource"},
						Version:         TurbineBuilds.Version{"version": "1"},
						DestinationPath: "some-resource",
						ConfigPath:      "build.yml",
					},
				}
			})

			It("uses them for the build's inputs", func() {
				turbineServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/builds"),
						successfulBuildStart(expectedTurbineBuild),
					),
				)

				_, err := builder.Start(job, build, map[string]builds.Version{
					"some-resource": builds.Version{"version": "1"},
				})
				Ω(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when the build is aborted while starting", func() {
			It("aborts the build on the turbine", func() {
				turbineServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/builds"),
						func(w http.ResponseWriter, r *http.Request) {
							err := redis.AbortBuild(job.Name, 1)
							Ω(err).ShouldNot(HaveOccurred())
						},
						successfulBuildStart(expectedTurbineBuild),
					),
					ghttp.VerifyRequest("POST", "/abort/the/build"),
				)

				_, err := builder.Start(job, build, nil)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(turbineServer.ReceivedRequests()).Should(HaveLen(2))
			})
		})

		Context("when the job has a resource that depends on other jobs", func() {
			BeforeEach(func() {
				job.Inputs = append(job.Inputs, config.Input{
					Resource: "some-dependant-resource",
					Passed:   []string{"job1", "job2"},
				})

				expectedTurbineBuild.Inputs = []TurbineBuilds.Input{
					{
						Name:            "some-resource",
						Type:            "git",
						Source:          TurbineBuilds.Source{"uri": "git://some-resource"},
						DestinationPath: "some-resource",
						ConfigPath:      "build.yml",
					},
					{
						Name:            "some-dependant-resource",
						Type:            "git",
						Source:          TurbineBuilds.Source{"uri": "git://some-dependant-resource"},
						Version:         TurbineBuilds.Version{"version": "1"},
						DestinationPath: "some-dependant-resource",
					},
				}
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
					turbineServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/builds"),
							successfulBuildStart(expectedTurbineBuild),
						),
					)

					build, err := builder.Start(job, build, nil)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(build.ID).Should(Equal(1))
				})
			})

			Context("and the other jobs do not satisfy the dependency", func() {
				It("returns an error", func() {
					_, err := builder.Start(job, build, nil)
					Ω(err).Should(HaveOccurred())
				})

				It("does not start the build", func() {
					build, err := redis.GetBuild(job.Name, build.ID)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(build.Status).Should(Equal(builds.StatusPending))
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
				_, err := builder.Start(job, build, nil)
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
				_, err := builder.Start(job, build, nil)
				Ω(err).Should(HaveOccurred())
			})
		})

		Context("when the turbine server is unreachable", func() {
			BeforeEach(func() {
				turbineServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/builds"),
						func(w http.ResponseWriter, r *http.Request) {
							turbineServer.HTTPTestServer.CloseClientConnections()
						},
					),
				)
			})

			It("returns an error", func() {
				_, err := builder.Start(job, build, nil)
				Ω(err).Should(HaveOccurred())

				Ω(turbineServer.ReceivedRequests()).Should(HaveLen(1))
			})
		})

		Context("when the turbine server returns non-201", func() {
			BeforeEach(func() {
				turbineServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/builds"),
						ghttp.RespondWith(400, ""),
					),
				)
			})

			It("returns an error", func() {
				_, err := builder.Start(job, build, nil)
				Ω(err).Should(HaveOccurred())
			})
		})
	})
})
