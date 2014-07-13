package resources_test

import (
	"net/http"
	"time"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	. "github.com/concourse/atc/resources"
	TurbineBuilds "github.com/concourse/turbine/api/builds"
	"github.com/concourse/turbine/routes"
	"github.com/gorilla/websocket"
	"github.com/tedsuo/rata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("TurbineChecker", func() {
	var turbineServer *ghttp.Server
	var pingInterval time.Duration
	var checker Checker

	var checkedInputs chan TurbineBuilds.Input
	var checkVersions chan []TurbineBuilds.Version
	var serverPings chan string
	var respondToPings chan struct{}

	var resource config.Resource

	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,

		CheckOrigin: func(*http.Request) bool {
			return true
		},
	}

	BeforeEach(func() {
		checkedInputs = make(chan TurbineBuilds.Input, 100)
		checkVersions = make(chan []TurbineBuilds.Version, 100)
		serverPings = make(chan string)
		respondToPings = make(chan struct{})

		turbineServer = ghttp.NewServer()
		pingInterval = 100 * time.Millisecond

		turbineServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/checks/stream"),
				func(w http.ResponseWriter, r *http.Request) {
					ws, err := upgrader.Upgrade(w, r, nil)
					Ω(err).ShouldNot(HaveOccurred())

					go func() {
						defer ws.Close()

						var input TurbineBuilds.Input

						for {
							err := ws.ReadJSON(&input)
							if err != nil {
								break
							}

							checkedInputs <- input

							err = ws.WriteJSON(<-checkVersions)
							if err != nil {
								break
							}
						}
					}()
				},
			),
		)

		checker = NewTurbineChecker(
			rata.NewRequestGenerator(turbineServer.URL(), routes.Routes),
			pingInterval,
		)

		resource = config.Resource{
			Name:   "some-input",
			Type:   "git",
			Source: config.Source{"uri": "http://example.com"},
		}
	})

	AfterEach(func() {
		turbineServer.HTTPTestServer.CloseClientConnections()
		turbineServer.Close()
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
