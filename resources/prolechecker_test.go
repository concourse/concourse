package resources_test

import (
	"encoding/json"

	"code.google.com/p/go.net/websocket"

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

	var checkedInputs chan ProleBuilds.Input
	var checkVersions chan []ProleBuilds.Version

	var resource config.Resource

	BeforeEach(func() {
		checkedInputs = make(chan ProleBuilds.Input, 100)
		checkVersions = make(chan []ProleBuilds.Version, 100)

		proleServer = ghttp.NewServer()

		proleServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/checks/stream"),
				websocket.Server{
					Handler: func(conn *websocket.Conn) {
						decoder := json.NewDecoder(conn)
						encoder := json.NewEncoder(conn)

						var input ProleBuilds.Input

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
			checkVersions <- []ProleBuilds.Version{
				ProleBuilds.Version{"ver": "abc"},
				ProleBuilds.Version{"ver": "def"},
			}

			checkVersions <- []ProleBuilds.Version{
				ProleBuilds.Version{"ver": "ghi"},
			}
		})

		It("returns each detected version", func() {
			Ω(checker.CheckResource(resource, nil)).Should(Equal([]builds.Version{
				builds.Version{"ver": "abc"},
				builds.Version{"ver": "def"},
			}))

			Ω(checkedInputs).Should(Receive(Equal(ProleBuilds.Input{
				Type:   resource.Type,
				Source: ProleBuilds.Source{"uri": "http://example.com"},
			})))

			Ω(checker.CheckResource(resource, builds.Version{"ver": "def"})).Should(Equal([]builds.Version{
				builds.Version{"ver": "ghi"},
			}))

			Ω(checkedInputs).Should(Receive(Equal(ProleBuilds.Input{
				Type:    resource.Type,
				Source:  ProleBuilds.Source{"uri": "http://example.com"},
				Version: ProleBuilds.Version{"ver": "def"},
			})))
		})
	})
})
