package builder_test

import (
	"net/http"
	"net/url"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	ProleBuilds "github.com/winston-ci/prole/api/builds"
	ProleRoutes "github.com/winston-ci/prole/routes"

	WinstonRoutes "github.com/winston-ci/winston/api/routes"
	. "github.com/winston-ci/winston/builder"
	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/db"
	"github.com/winston-ci/winston/endpoint"
	"github.com/winston-ci/winston/jobs"
	"github.com/winston-ci/winston/redisrunner"
	"github.com/winston-ci/winston/resources"
)

var _ = Describe("Builder", func() {
	var redisRunner *redisrunner.Runner
	var redis db.DB

	var proleServer *ghttp.Server

	var builder Builder

	var job jobs.Job

	BeforeEach(func() {
		redisRunner = redisrunner.NewRunner()
		redisRunner.Start()

		redis = db.NewRedis(redisRunner.Pool())

		proleServer = ghttp.NewServer()

		proleURL, err := url.Parse(proleServer.URL())
		Ω(err).ShouldNot(HaveOccurred())

		winstonURL, err := url.Parse("http://winston-server")
		Ω(err).ShouldNot(HaveOccurred())

		job = jobs.Job{
			Name: "foo",

			BuildConfigPath: "some-build/build.yml",

			Inputs: []resources.Resource{
				{
					Name: "some-resource",

					Type: "git",
					URI:  "git://example.com/foo/repo.git",
				},
			},
		}

		builder = NewBuilder(
			redis,
			endpoint.EndpointRoutes{
				URL:    proleURL,
				Routes: ProleRoutes.Routes,
			},
			endpoint.EndpointRoutes{
				URL:    winstonURL,
				Routes: WinstonRoutes.Routes,
			},
		)
	})

	AfterEach(func() {
		redisRunner.Stop()
	})

	It("triggers a build on the prole endpoint", func() {
		proleServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/builds"),
				ghttp.VerifyJSONRepresenting(ProleBuilds.Build{
					ConfigPath: "some-build/build.yml",

					Callback: "http://winston-server/builds/foo/1/result",
					LogsURL:  "ws://winston-server/builds/foo/1/log/input",

					Source: ProleBuilds.BuildSource{
						Type:   "git",
						URI:    "git://example.com/foo/repo.git",
						Branch: "master",
						Ref:    "HEAD",
					},
				}),
				ghttp.RespondWith(201, ""),
			),
		)

		build, err := builder.Build(job)
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

		build, err := builder.Build(job)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(build.ID).Should(Equal(1))

		build, err = builder.Build(job)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(build.ID).Should(Equal(2))
	})

	It("marks the build as running", func() {
		proleServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/builds"),
				ghttp.RespondWith(201, ""),
			),
		)

		build, err := builder.Build(job)
		Ω(err).ShouldNot(HaveOccurred())

		build, err = redis.GetBuild(job.Name, build.ID)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(build.State).Should(Equal(builds.BuildStateRunning))
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
			_, err := builder.Build(job)
			Ω(err).Should(HaveOccurred())
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
			_, err := builder.Build(job)
			Ω(err).Should(HaveOccurred())
		})
	})
})
