package resources_test

import (
	"github.com/tedsuo/router"
	ProleBuilds "github.com/winston-ci/prole/api/builds"
	"github.com/winston-ci/prole/routes"
	"github.com/winston-ci/winston/builds"
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
			Source: config.Source{"uri": "http://example.com"},
		}
	})

	Context("when the endpoint returns new versions", func() {
		BeforeEach(func() {
			returnedVersions1 := []ProleBuilds.Version{
				ProleBuilds.Version{"ver": "abc"},
				ProleBuilds.Version{"ver": "def"},
			}

			returnedVersions2 := []ProleBuilds.Version{
				ProleBuilds.Version{"ver": "ghi"},
			}

			proleServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/checks"),
					ghttp.VerifyJSONRepresenting(ProleBuilds.Input{
						Type:    resource.Type,
						Source:  ProleBuilds.Source{"uri": "http://example.com"},
						Version: ProleBuilds.Version{"ver": "1"},
					}),
					ghttp.RespondWithJSONEncoded(200, returnedVersions1),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/checks"),
					ghttp.VerifyJSONRepresenting(ProleBuilds.Input{
						Type:   resource.Type,
						Source: ProleBuilds.Source{"uri": "http://example.com"},
					}),
					ghttp.RespondWithJSONEncoded(200, returnedVersions2),
				),
			)
		})

		It("returns each detected version", func() {
			Ω(checker.CheckResource(resource, builds.Version{"ver": "1"})).Should(Equal([]builds.Version{
				builds.Version{"ver": "abc"},
				builds.Version{"ver": "def"},
			}))

			Ω(checker.CheckResource(resource, nil)).Should(Equal([]builds.Version{
				builds.Version{"ver": "ghi"},
			}))
		})
	})
})
