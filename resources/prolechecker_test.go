package resources_test

import (
	"github.com/tedsuo/router"
	"github.com/winston-ci/prole/api/builds"
	"github.com/winston-ci/prole/routes"
	"github.com/winston-ci/winston/config"
	. "github.com/winston-ci/winston/resources"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ProleChecker", func() {
	var proleServer *ghttp.Server
	var checker Checker

	var resource config.Resource

	BeforeEach(func() {
		proleServer = ghttp.NewServer()
		proleServer.AllowUnhandledRequests = true

		checker = NewProleChecker(
			router.NewRequestGenerator(proleServer.URL(), routes.Routes),
		)

		resource = config.Resource{
			Name:   "some-input",
			Type:   "git",
			Source: config.Source("123"),
		}
	})

	Context("when the endpoint returns new sources", func() {
		BeforeEach(func() {
			returnedSources1 := []builds.Source{
				builds.Source(`"abc"`),
				builds.Source(`"def"`),
			}

			returnedSources2 := []builds.Source{
				builds.Source(`"ghi"`),
			}

			proleServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/checks"),
					ghttp.VerifyJSONRepresenting(builds.Input{
						Type:   resource.Type,
						Source: builds.Source("123"),
					}),
					ghttp.RespondWithJSONEncoded(200, returnedSources1),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/checks"),
					ghttp.RespondWithJSONEncoded(200, returnedSources2),
				),
			)
		})

		It("returns the resource with each detected source", func() {
			Ω(checker.CheckResource(resource)).Should(Equal([]config.Resource{
				{
					Name:   "some-input",
					Type:   "git",
					Source: config.Source(`"abc"`),
				},
				{
					Name:   "some-input",
					Type:   "git",
					Source: config.Source(`"def"`),
				},
			}))
		})
	})
})
