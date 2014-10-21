package builder_test

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"

	tbuilds "github.com/concourse/turbine/api/builds"
	TurbineRoutes "github.com/concourse/turbine/routes"

	. "github.com/concourse/atc/builder"
	"github.com/concourse/atc/builder/fakes"
	"github.com/concourse/atc/builds"
)

var _ = Describe("Builder", func() {
	var (
		db            *fakes.FakeBuilderDB
		turbineServer *ghttp.Server

		builder Builder

		build        builds.Build
		turbineBuild tbuilds.Build
	)

	BeforeEach(func() {
		db = new(fakes.FakeBuilderDB)

		turbineServer = ghttp.NewServer()

		build = builds.Build{
			ID:   128,
			Name: "some-build",
		}

		turbineBuild = tbuilds.Build{
			Config: tbuilds.Config{
				Image: "some-image",

				Params: map[string]string{
					"FOO": "1",
					"BAR": "2",
				},

				Run: tbuilds.RunConfig{
					Path: "some-script",
					Args: []string{"arg1", "arg2"},
				},
			},
		}

		db.StartBuildReturns(true, nil)

		builder = NewBuilder(
			db,
			rata.NewRequestGenerator(turbineServer.URL(), TurbineRoutes.Routes),
		)
	})

	successfulBuildStart := func(build tbuilds.Build) http.HandlerFunc {
		createdBuild := build
		createdBuild.Guid = "some-build-guid"

		return ghttp.CombineHandlers(
			ghttp.VerifyJSONRepresenting(build),
			func(w http.ResponseWriter, r *http.Request) {
				w.Header().Add("X-Turbine-Endpoint", turbineServer.URL())
			},
			ghttp.RespondWithJSONEncoded(201, createdBuild),
		)
	}

	It("starts the build and saves its guid and endpoint", func() {
		turbineServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/builds"),
				successfulBuildStart(turbineBuild),
			),
		)

		err := builder.Build(build, turbineBuild)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(db.StartBuildCallCount()).Should(Equal(1))

		buildID, guid, endpoint := db.StartBuildArgsForCall(0)
		Ω(buildID).Should(Equal(128))
		Ω(guid).Should(ContainSubstring("some-build-guid"))
		Ω(endpoint).Should(ContainSubstring(turbineServer.URL()))
	})

	Context("when the build fails to transition to started", func() {
		BeforeEach(func() {
			db.StartBuildReturns(false, nil)
		})

		It("aborts the build on the turbine", func() {
			turbineServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/builds"),
					successfulBuildStart(turbineBuild),
				),
				ghttp.VerifyRequest("POST", "/builds/some-build-guid/abort"),
			)

			err := builder.Build(build, turbineBuild)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(turbineServer.ReceivedRequests()).Should(HaveLen(2))
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
			err := builder.Build(build, turbineBuild)
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
			err := builder.Build(build, turbineBuild)
			Ω(err).Should(HaveOccurred())
		})
	})
})
