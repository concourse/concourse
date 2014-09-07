package builder_test

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"

	TurbineBuilds "github.com/concourse/turbine/api/builds"
	TurbineRoutes "github.com/concourse/turbine/routes"

	. "github.com/concourse/atc/builder"
	"github.com/concourse/atc/builder/fakes"
	"github.com/concourse/atc/builds"
	CallbacksRoutes "github.com/concourse/atc/callbacks/routes"
	"github.com/concourse/atc/config"
)

var _ = Describe("Builder", func() {
	var db *fakes.FakeBuilderDB

	var turbineServer *ghttp.Server

	var build builds.Build

	var builder Builder

	var job config.Job
	var resources config.Resources

	var expectedTurbineBuild TurbineBuilds.Build

	BeforeEach(func() {
		db = new(fakes.FakeBuilderDB)

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

			StatusCallback: "http://atc-server/builds/128",
			EventsCallback: "ws://atc-server/builds/128/events",
		}

		builder = NewBuilder(
			db,
			resources,
			rata.NewRequestGenerator(turbineServer.URL(), TurbineRoutes.Routes),
			rata.NewRequestGenerator("http://atc-server", CallbacksRoutes.Routes),
		)

		build = builds.Build{
			ID:   128,
			Name: "some-build",
		}

		db.StartBuildReturns(true, nil)
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

		Ω(db.StartBuildCallCount()).Should(Equal(1))

		buildID, abortURL := db.StartBuildArgsForCall(0)
		Ω(buildID).Should(Equal(128))
		Ω(abortURL).Should(ContainSubstring("/abort/the/build"))
	})

	Context("when the build fails to transition to started", func() {
		BeforeEach(func() {
			db.StartBuildReturns(false, nil)
		})

		It("aborts the build on the turbine", func() {
			turbineServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/builds"),
					successfulBuildStart(expectedTurbineBuild),
				),
				ghttp.VerifyRequest("POST", "/abort/the/build"),
			)

			err := builder.Build(build, job, nil)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(turbineServer.ReceivedRequests()).Should(HaveLen(2))
		})
	})

	Context("when the build has outputs", func() {
		BeforeEach(func() {
			job.Outputs = []config.Output{
				{
					Resource: "some-resource",
					Params:   config.Params{"foo": "bar"},
				},
				{
					Resource: "some-resource",
					Params:   config.Params{"foo": "bar"},
					On:       []config.OutputCondition{"failure"},
				},
				{
					Resource: "some-resource",
					Params:   config.Params{"foo": "bar"},
					On:       []config.OutputCondition{},
				},
			}

			expectedTurbineBuild.Outputs = []TurbineBuilds.Output{
				{
					Name:   "some-resource",
					Type:   "git",
					On:     []TurbineBuilds.OutputCondition{TurbineBuilds.OutputConditionSuccess},
					Params: TurbineBuilds.Params{"foo": "bar"},
					Source: TurbineBuilds.Source{"uri": "git://some-resource"},
				},
				{
					Name:   "some-resource",
					Type:   "git",
					On:     []TurbineBuilds.OutputCondition{TurbineBuilds.OutputConditionFailure},
					Params: TurbineBuilds.Params{"foo": "bar"},
					Source: TurbineBuilds.Source{"uri": "git://some-resource"},
				},
				{
					Name:   "some-resource",
					Type:   "git",
					On:     []TurbineBuilds.OutputCondition{},
					Params: TurbineBuilds.Params{"foo": "bar"},
					Source: TurbineBuilds.Source{"uri": "git://some-resource"},
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
					Source:  builds.Source{"uri": "git://some-provided-uri"},
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
