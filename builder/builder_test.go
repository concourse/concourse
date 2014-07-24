package builder_test

import (
	"database/sql"
	"net/http"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/rata"

	TurbineBuilds "github.com/concourse/turbine/api/builds"
	TurbineRoutes "github.com/concourse/turbine/routes"

	WinstonRoutes "github.com/concourse/atc/api/routes"
	. "github.com/concourse/atc/builder"
	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	Db "github.com/concourse/atc/db"
	"github.com/concourse/atc/postgresrunner"
)

var _ = Describe("Builder", func() {
	var postgresRunner postgresrunner.Runner

	var dbConn *sql.DB
	var dbProcess ifrit.Process

	var db Db.DB

	var turbineServer *ghttp.Server

	var builder Builder

	var job config.Job
	var resources config.Resources

	var expectedTurbineBuild TurbineBuilds.Build

	BeforeSuite(func() {
		postgresRunner = postgresrunner.Runner{
			Port: 5433 + GinkgoParallelNode(),
		}

		dbProcess = ifrit.Envoke(postgresRunner)
	})

	AfterSuite(func() {
		dbProcess.Signal(os.Interrupt)
		Eventually(dbProcess.Wait(), 10*time.Second).Should(Receive())
	})

	BeforeEach(func() {
		var err error

		postgresRunner.CreateTestDB()

		dbConn = postgresRunner.Open()
		db = Db.NewSQL(dbConn)

		turbineServer = ghttp.NewServer()

		job = config.Job{
			Name: "some-job",

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
					Params:   config.Params{"some": "params"},
				},
			},
		}

		err = db.RegisterJob("some-job")
		Ω(err).ShouldNot(HaveOccurred())

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

		err = db.RegisterResource("some-resource")
		Ω(err).ShouldNot(HaveOccurred())

		err = db.RegisterResource("some-dependant-resource")
		Ω(err).ShouldNot(HaveOccurred())

		err = db.RegisterResource("some-output-resource")
		Ω(err).ShouldNot(HaveOccurred())

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

			Inputs: []TurbineBuilds.Input{
				{
					Name:       "some-resource",
					Type:       "git",
					Source:     TurbineBuilds.Source{"uri": "git://some-resource"},
					Params:     TurbineBuilds.Params{"some": "params"},
					ConfigPath: "build.yml",
				},
			},

			Outputs: []TurbineBuilds.Output{},

			Privileged: true,

			Callback: "http://atc-server/builds/some-job/1",
			LogsURL:  "ws://atc-server/builds/some-job/1/log/input",
		}

		builder = NewBuilder(
			db,
			resources,
			rata.NewRequestGenerator(turbineServer.URL(), TurbineRoutes.Routes),
			rata.NewRequestGenerator("http://atc-server", WinstonRoutes.Routes),
		)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Ω(err).ShouldNot(HaveOccurred())

		postgresRunner.DropTestDB()
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

		resource := config.Resource{Name: "some-resource"}
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
					Resource: "some-resource",
				})

				job.Outputs = append(job.Outputs, config.Output{
					Resource: "some-resource",
				})
			})

			Context("and a build is running", func() {
				BeforeEach(func() {
					runningBuild, err := db.CreateBuild(job.Name)
					Ω(err).ShouldNot(HaveOccurred())

					scheduled, err := db.ScheduleBuild(job.Name, runningBuild.ID, true)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeTrue())

					err = db.SaveBuildInput(job.Name, runningBuild.ID, builds.VersionedResource{
						Name:    "some-resource",
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

					existingBuild, err = db.CreateBuild(job.Name)
					Ω(err).ShouldNot(HaveOccurred())

					scheduled, err := db.ScheduleBuild(job.Name, existingBuild.ID, false)
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

						existingBuild, err = db.CreateBuild(job.Name)
						Ω(err).ShouldNot(HaveOccurred())

						scheduled, err := db.ScheduleBuild(job.Name, existingBuild.ID, false)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeTrue())

						err = db.SaveBuildStatus(job.Name, existingBuild.ID, status)
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
						Name:       "some-resource",
						Type:       "git",
						Source:     TurbineBuilds.Source{"uri": "git://some-resource"},
						Params:     TurbineBuilds.Params{"some": "params"},
						Version:    TurbineBuilds.Version{"version": "1"},
						ConfigPath: "build.yml",
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
							err := db.AbortBuild(job.Name, 1)
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
						Name:       "some-resource",
						Type:       "git",
						Source:     TurbineBuilds.Source{"uri": "git://some-resource"},
						Params:     TurbineBuilds.Params{"some": "params"},
						ConfigPath: "build.yml",
					},
					{
						Name:    "some-dependant-resource",
						Type:    "git",
						Source:  TurbineBuilds.Source{"uri": "git://some-dependant-resource"},
						Version: TurbineBuilds.Version{"version": "1"},
					},
				}
			})

			Context("and the other jobs satisfy the dependency", func() {
				BeforeEach(func() {
					err := db.RegisterJob("job1")
					Ω(err).ShouldNot(HaveOccurred())

					err = db.RegisterJob("job2")
					Ω(err).ShouldNot(HaveOccurred())

					j1b1, err := db.CreateBuild("job1")
					Ω(err).ShouldNot(HaveOccurred())

					j1b2, err := db.CreateBuild("job1")
					Ω(err).ShouldNot(HaveOccurred())

					j2b1, err := db.CreateBuild("job2")
					Ω(err).ShouldNot(HaveOccurred())

					err = db.SaveBuildOutput("job1", j1b1.ID, builds.VersionedResource{
						Name:    "some-dependant-resource",
						Version: builds.Version{"version": "1"},
					})
					Ω(err).ShouldNot(HaveOccurred())

					err = db.SaveBuildOutput("job2", j2b1.ID, builds.VersionedResource{
						Name:    "some-dependant-resource",
						Version: builds.Version{"version": "1"},
					})
					Ω(err).ShouldNot(HaveOccurred())

					err = db.SaveBuildOutput("job1", j1b2.ID, builds.VersionedResource{
						Name:    "some-dependant-resource",
						Version: builds.Version{"version": "2"},
					})
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
					build, err := db.GetBuild(job.Name, build.ID)
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
