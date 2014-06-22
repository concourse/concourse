package resources_test

import (
	"encoding/json"

	"code.google.com/p/go.net/websocket"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	. "github.com/concourse/atc/resources"
	TurbineBuilds "github.com/concourse/turbine/api/builds"
	"github.com/concourse/turbine/routes"
	"github.com/tedsuo/router"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("TurbineChecker", func() {
	var turbineServer *ghttp.Server
	var checker Checker

	var checkedInputs chan TurbineBuilds.Input
	var checkVersions chan []TurbineBuilds.Version

	var resource config.Resource

	BeforeEach(func() {
		checkedInputs = make(chan TurbineBuilds.Input, 100)
		checkVersions = make(chan []TurbineBuilds.Version, 100)

		turbineServer = ghttp.NewServer()

		turbineServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/checks/stream"),
				websocket.Server{
					Handler: func(conn *websocket.Conn) {
						decoder := json.NewDecoder(conn)
						encoder := json.NewEncoder(conn)

						var input TurbineBuilds.Input

						for {
							err := decoder.Decode(&input)
							Ω(err).ShouldNot(HaveOccurred())

							checkedInputs <- input

							err = encoder.Encode(<-checkVersions)
							Ω(err).ShouldNot(HaveOccurred())
						}
					},
				}.ServeHTTP,
			),
		)

		checker = NewTurbineChecker(
			router.NewRequestGenerator(turbineServer.URL(), routes.Routes),
		)

		resource = config.Resource{
			Name:   "some-input",
			Type:   "git",
			Source: config.Source{"uri": "http://example.com"},
		}
	})

	Context("when the endpoint returns new versions", func() {
		BeforeEach(func() {
			checkVersions <- []TurbineBuilds.Version{
				TurbineBuilds.Version{"ver": "abc"},
				TurbineBuilds.Version{"ver": "def"},
			}

			checkVersions <- []TurbineBuilds.Version{
				TurbineBuilds.Version{"ver": "ghi"},
			}
		})

		It("returns each detected version", func() {
			Ω(checker.CheckResource(resource, nil)).Should(Equal([]builds.Version{
				builds.Version{"ver": "abc"},
				builds.Version{"ver": "def"},
			}))

			Ω(checkedInputs).Should(Receive(Equal(TurbineBuilds.Input{
				Type:   resource.Type,
				Source: TurbineBuilds.Source{"uri": "http://example.com"},
			})))

			Ω(checker.CheckResource(resource, builds.Version{"ver": "def"})).Should(Equal([]builds.Version{
				builds.Version{"ver": "ghi"},
			}))

			Ω(checkedInputs).Should(Receive(Equal(TurbineBuilds.Input{
				Type:    resource.Type,
				Source:  TurbineBuilds.Source{"uri": "http://example.com"},
				Version: TurbineBuilds.Version{"ver": "def"},
			})))
		})
	})
})
