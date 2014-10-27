package builder_test

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"

	. "github.com/concourse/atc/builder"
	"github.com/concourse/atc/builder/fakes"
	"github.com/concourse/atc/db"
	"github.com/concourse/turbine"
)

var _ = Describe("Builder", func() {
	var (
		builderDB     *fakes.FakeBuilderDB
		turbineServer *ghttp.Server

		builder Builder

		build        db.Build
		turbineBuild turbine.Build
	)

	BeforeEach(func() {
		builderDB = new(fakes.FakeBuilderDB)

		turbineServer = ghttp.NewServer()

		build = db.Build{
			ID:   128,
			Name: "some-build",
		}

		turbineBuild = turbine.Build{
			Config: turbine.Config{
				Image: "some-image",

				Params: map[string]string{
					"FOO": "1",
					"BAR": "2",
				},

				Run: turbine.RunConfig{
					Path: "some-script",
					Args: []string{"arg1", "arg2"},
				},
			},
		}

		builderDB.StartBuildReturns(true, nil)

		builder = NewBuilder(
			builderDB,
			rata.NewRequestGenerator(turbineServer.URL(), turbine.Routes),
		)
	})

	successfulBuildStart := func(build turbine.Build) http.HandlerFunc {
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

		Ω(builderDB.StartBuildCallCount()).Should(Equal(1))

		buildID, guid, endpoint := builderDB.StartBuildArgsForCall(0)
		Ω(buildID).Should(Equal(128))
		Ω(guid).Should(ContainSubstring("some-build-guid"))
		Ω(endpoint).Should(ContainSubstring(turbineServer.URL()))
	})

	Context("when the build fails to transition to started", func() {
		BeforeEach(func() {
			builderDB.StartBuildReturns(false, nil)
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
