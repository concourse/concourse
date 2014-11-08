package radar_test

import (
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/concourse/atc/radar"
	"github.com/concourse/turbine"
	"github.com/gorilla/websocket"
	"github.com/tedsuo/rata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("TurbineChecker", func() {
	var turbineServer *ghttp.Server
	var checker ResourceChecker

	var checkedInputs chan turbine.Input
	var checkVersions chan []turbine.Version

	var resource atc.ResourceConfig

	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,

		CheckOrigin: func(*http.Request) bool {
			return true
		},
	}

	BeforeEach(func() {
		checkedInputs = make(chan turbine.Input, 100)
		checkVersions = make(chan []turbine.Version, 100)

		turbineServer = ghttp.NewServer()

		turbineServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/checks/stream"),
				func(w http.ResponseWriter, r *http.Request) {
					ws, err := upgrader.Upgrade(w, r, nil)
					Ω(err).ShouldNot(HaveOccurred())

					go func() {
						defer ws.Close()

						var input turbine.Input

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
			rata.NewRequestGenerator(turbineServer.URL(), turbine.Routes),
		)

		resource = atc.ResourceConfig{
			Name:   "some-resource",
			Type:   "git",
			Source: atc.Source{"uri": "http://example.com"},
		}
	})

	AfterEach(func() {
		turbineServer.HTTPTestServer.CloseClientConnections()
		turbineServer.Close()
	})

	Context("when the endpoint returns new versions", func() {
		BeforeEach(func() {
			checkVersions <- []turbine.Version{
				turbine.Version{"ver": "abc"},
				turbine.Version{"ver": "def"},
			}

			checkVersions <- []turbine.Version{
				turbine.Version{"ver": "ghi"},
			}
		})

		It("returns each detected version", func() {
			Ω(checker.CheckResource(resource, nil)).Should(Equal([]db.Version{
				db.Version{"ver": "abc"},
				db.Version{"ver": "def"},
			}))

			Ω(checkedInputs).Should(Receive(Equal(turbine.Input{
				Resource: "some-resource",
				Type:     resource.Type,
				Source:   turbine.Source{"uri": "http://example.com"},
			})))

			Ω(checker.CheckResource(resource, db.Version{"ver": "def"})).Should(Equal([]db.Version{
				db.Version{"ver": "ghi"},
			}))

			Ω(checkedInputs).Should(Receive(Equal(turbine.Input{
				Resource: "some-resource",
				Type:     resource.Type,
				Source:   turbine.Source{"uri": "http://example.com"},
				Version:  turbine.Version{"ver": "def"},
			})))
		})
	})
})
