package hijacker_test

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/rata"

	"github.com/gorilla/websocket"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/hijacker"
)

var _ = Describe("Hijacker", func() {
	// Other functionality tested through the hijack command integration test.

	upgrader := websocket.Upgrader{}

	wasPingedHandler := func(id string, didHijack chan<- struct{}, didGetPinged chan<- struct{}) http.HandlerFunc {
		return ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", fmt.Sprintf("/api/v1/teams/some-team/containers/%s/hijack", id)),
			func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()

				close(didHijack)
				conn, err := upgrader.Upgrade(w, r, nil)
				Expect(err).NotTo(HaveOccurred())

				defer conn.Close()

				conn.SetPingHandler(func(data string) error {
					conn.WriteControl(websocket.CloseMessage, []byte{}, time.Now().Add(time.Second))
					close(didGetPinged)
					return nil
				})

				for {
					_, _, err := conn.ReadMessage()
					if err != nil {
						break
					}
				}

			},
		)
	}

	var (
		server *ghttp.Server

		didHijack  chan struct{}
		didGetPing chan struct{}
	)

	BeforeEach(func() {
		didHijack = make(chan struct{})
		didGetPing = make(chan struct{})

		server = ghttp.NewServer()
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("keeping the connection alive", func() {
		BeforeEach(func() {
			server.AppendHandlers(wasPingedHandler("hello", didHijack, didGetPing))
		})

		It("sends the occasional ping", func() {
			tlsConfig := &tls.Config{
				InsecureSkipVerify: true,
			}

			reqGenerator := rata.NewRequestGenerator(server.URL(), atc.Routes)

			stdin := gbytes.NewBuffer()
			stdout := gbytes.NewBuffer()
			stderr := gbytes.NewBuffer()

			h := hijacker.New(tlsConfig, reqGenerator, nil)
			_, err := h.Hijack("some-team", "hello", atc.HijackProcessSpec{
				Path: "/bin/echo",
				Args: []string{"hello", "world"},
			}, hijacker.ProcessIO{
				In:  stdin,
				Out: stdout,
				Err: stderr,
			})

			h.SetHeartbeatInterval(100 * time.Millisecond)

			Expect(err).ShouldNot(HaveOccurred())
			Expect(didHijack).To(BeClosed())
			Eventually(didGetPing).Should(BeClosed())
		})
	})
})
