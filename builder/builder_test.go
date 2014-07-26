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

	var build builds.Build

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

		build, err = db.CreateBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())
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

	It("starts the build and saves its abort url", func() {
		turbineServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/builds"),
				successfulBuildStart(expectedTurbineBuild),
			),
		)

		err := builder.Build(build, job, nil)
		Ω(err).ShouldNot(HaveOccurred())

		startedBuild, err := db.GetBuild(job.Name, build.ID)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(startedBuild.AbortURL).Should(ContainSubstring("/abort/the/build"))
		Ω(startedBuild.Status).Should(Equal(builds.StatusStarted))
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

			err := builder.Build(build, job, nil)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(turbineServer.ReceivedRequests()).Should(HaveLen(2))
		})
	})

	Context("when the job is serial", func() {
		BeforeEach(func() {
			job.Serial = true
		})

		Context("and the current build is scheduled", func() {
			var newBuild builds.Build

			BeforeEach(func() {
				var err error

				scheduled, err := db.ScheduleBuild(job.Name, build.ID, false)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(scheduled).Should(BeTrue())

				newBuild, err = db.CreateBuild(job.Name)
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("leaves the build pending", func() {
				err := builder.Build(newBuild, job, nil)
				Ω(err).ShouldNot(HaveOccurred())

				queuedBuild, err := db.GetBuild(job.Name, newBuild.ID)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(queuedBuild.Status).Should(Equal(builds.StatusPending))
			})

			It("does not trigger a build", func() {
				err := builder.Build(newBuild, job, nil)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(turbineServer.ReceivedRequests()).Should(BeEmpty())
			})
		})

		for _, s := range []builds.Status{builds.StatusSucceeded, builds.StatusFailed, builds.StatusErrored} {
			status := s

			Context("and the current build is "+string(status), func() {
				BeforeEach(func() {
					var err error

					scheduled, err := db.ScheduleBuild(job.Name, build.ID, false)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeTrue())

					err = db.SaveBuildStatus(job.Name, build.ID, status)
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("starts the build", func() {
					expectedTurbineBuild.Callback = "http://atc-server/builds/some-job/2"
					expectedTurbineBuild.LogsURL = "ws://atc-server/builds/some-job/2/log/input"

					turbineServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/builds"),
							successfulBuildStart(expectedTurbineBuild),
						),
					)

					newBuild, err := db.CreateBuild(job.Name)
					Ω(err).ShouldNot(HaveOccurred())

					err = builder.Build(newBuild, job, nil)
					Ω(err).ShouldNot(HaveOccurred())

					queuedBuild, err := db.GetBuild(job.Name, newBuild.ID)
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

			err := builder.Build(build, job, nil)
			Ω(err).ShouldNot(HaveOccurred())
		})
	})

	Context("when versioned resources are specified", func() {
		BeforeEach(func() {
			expectedTurbineBuild.Inputs = []TurbineBuilds.Input{
				{
					Name:       "some-resource",
					Type:       "git-ng",
					Source:     TurbineBuilds.Source{"uri": "git://some-provided-uri"},
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

			err := builder.Build(build, job, builds.VersionedResources{
				{
					Name:    "some-resource",
					Type:    "git-ng",
					Version: builds.Version{"version": "1"},
					Source:  config.Source{"uri": "git://some-provided-uri"},
				},
			})
			Ω(err).ShouldNot(HaveOccurred())
		})
	})

	Context("when the job's input is not found", func() {
		BeforeEach(func() {
			job.Inputs = append(job.Inputs, config.Input{
				Resource: "some-bogus-resource",
			})
		})

		It("returns an error", func() {
			err := builder.Build(build, job, nil)
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
			err := builder.Build(build, job, nil)
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
			err := builder.Build(build, job, nil)
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
			err := builder.Build(build, job, nil)
			Ω(err).Should(HaveOccurred())
		})
	})
})
