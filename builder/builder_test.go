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
					Name:   "some-resource",
					Type:   "git",
					Source: config.Source(`{"uri":"git://example.com/foo/repo.git"}`),
				},
			},
		}

		builder = NewBuilder(
			redis,
			router.NewRequestGenerator(proleServer.URL(), ProleRoutes.Routes),
			router.NewRequestGenerator("http://winston-server", WinstonRoutes.Routes),
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
					Privileged: true,

					Callback: "http://winston-server/builds/foo/1",
					LogsURL:  "ws://winston-server/builds/foo/1/log/input",

					Inputs: []ProleBuilds.Input{
						{
							Type: "git",

							Source: ProleBuilds.Source(`{"uri":"git://example.com/foo/repo.git"}`),

							DestinationPath: "some-resource",
							ConfigPath:      "build.yml",
						},
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
