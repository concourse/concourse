package logfanout_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/concourse/atc/logfanout"
	"github.com/gorilla/websocket"

	"github.com/concourse/atc/logfanout/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Logfanout", func() {
	var (
		logDB *fakes.FakeLogDB

		fanout *LogFanout
	)

	BeforeEach(func() {
		logDB = new(fakes.FakeLogDB)

		fanout = NewLogFanout("some-job", 1, logDB)
	})

	rawMSG := func(msg string) *json.RawMessage {
		payload := []byte(msg)
		return (*json.RawMessage)(&payload)
	}

	Describe("WriteMessage", func() {
		It("appends the message to the build's log", func() {
			err := fanout.WriteMessage(rawMSG("wat"))
			Ω(err).ShouldNot(HaveOccurred())
		})
	})

	Context("when a sink is attached", func() {
		var (
			dummyServer  *httptest.Server
			dummyAddr    string
			serverConnCh chan *websocket.Conn

			serverConn *websocket.Conn
			clientConn *websocket.Conn
		)

		var upgrader = websocket.Upgrader{
			CheckOrigin: func(*http.Request) bool {
				return true
			},
		}

		BeforeEach(func() {
			serverConnCh = make(chan *websocket.Conn)

			dummyServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var err error

				conn, err := upgrader.Upgrade(w, r, nil)
				Ω(err).ShouldNot(HaveOccurred())

				serverConnCh <- conn
			}))

			dummyAddr = dummyServer.Listener.Addr().String()

			var err error
			clientConn, _, err = (&websocket.Dialer{}).Dial("ws://"+dummyAddr, nil)
			Ω(err).ShouldNot(HaveOccurred())

			serverConn = <-serverConnCh
		})

		AfterEach(func() {
			serverConn.Close()
			clientConn.Close()
			dummyServer.Close()
		})

		JustBeforeEach(func() {
			err := fanout.Attach(serverConn)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Describe("writing messages", func() {
			It("writes them out to anyone attached", func() {
				err := fanout.WriteMessage(rawMSG(`{"hello":1}`))
				Ω(err).ShouldNot(HaveOccurred())

				var msg *json.RawMessage
				err = clientConn.ReadJSON(&msg)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(msg).Should(Equal(rawMSG(`{"hello":1}`)))
			})
		})

		Context("when there is a build log saved", func() {
			BeforeEach(func() {
				logDB.BuildLogReturns([]byte(`{"version":"1.0"}{"some":"saved log"}{"another":"message"}`), nil)
			})

			It("immediately sends its contents", func() {
				var msg *json.RawMessage

				err := clientConn.ReadJSON(&msg)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(msg).Should(Equal(rawMSG(`{"version":"1.0"}`)))

				err = clientConn.ReadJSON(&msg)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(msg).Should(Equal(rawMSG(`{"some":"saved log"}`)))

				err = clientConn.ReadJSON(&msg)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(msg).Should(Equal(rawMSG(`{"another":"message"}`)))
			})

			Context("but it contains pre-event stream output (backwards compatibility", func() {
				longLog := strings.Repeat("x", 1025)

				BeforeEach(func() {
					logDB.BuildLogReturns([]byte(longLog), nil)
				})

				type versionMessage struct {
					Version string `json:"version"`
				}

				It("writes a 0.0 version followed by the contents, chunked", func() {
					var version versionMessage
					err := clientConn.ReadJSON(&version)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(version.Version).Should(Equal("0.0"))

					typ, body, err := clientConn.ReadMessage()
					Ω(err).ShouldNot(HaveOccurred())
					Ω(typ).Should(Equal(websocket.TextMessage))
					Ω(string(body)).Should(Equal(longLog[0:1024]))

					typ, body, err = clientConn.ReadMessage()
					Ω(err).ShouldNot(HaveOccurred())
					Ω(typ).Should(Equal(websocket.TextMessage))
					Ω(string(body)).Should(Equal("x"))
				})

				Context("when a unicode codepoint falls on the chunk boundary", func() {
					unicodeLongLog := longLog[0:1023] + "日本語"

					BeforeEach(func() {
						logDB.BuildLogReturns([]byte(unicodeLongLog), nil)
					})

					It("does not cut it in half", func() {
						var version versionMessage
						err := clientConn.ReadJSON(&version)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(version.Version).Should(Equal("0.0"))

						typ, body, err := clientConn.ReadMessage()
						Ω(err).ShouldNot(HaveOccurred())
						Ω(typ).Should(Equal(websocket.TextMessage))
						Ω(string(body)).Should(Equal(unicodeLongLog))
					})
				})
			})

			Describe("closing", func() {
				BeforeEach(func() {
					err := fanout.Close()
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("flushes the log and immediately closes", func() {
					var msg *json.RawMessage

					err := clientConn.ReadJSON(&msg)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(msg).Should(Equal(rawMSG(`{"version":"1.0"}`)))

					err = clientConn.ReadJSON(&msg)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(msg).Should(Equal(rawMSG(`{"some":"saved log"}`)))

					err = clientConn.ReadJSON(&msg)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(msg).Should(Equal(rawMSG(`{"another":"message"}`)))

					err = clientConn.ReadJSON(&msg)
					Ω(err).Should(HaveOccurred())
				})
			})
		})

		Describe("closing", func() {
			It("disconnects attached sinks", func() {
				err := fanout.Close()
				Ω(err).ShouldNot(HaveOccurred())

				_, _, err = clientConn.ReadMessage()
				Ω(err).Should(HaveOccurred())
			})
		})

		Context("and another is attached", func() {
			var (
				secondServerConn *websocket.Conn
				secondClientConn *websocket.Conn
			)

			BeforeEach(func() {
				var err error
				secondClientConn, _, err = (&websocket.Dialer{}).Dial("ws://"+dummyAddr, nil)
				Ω(err).ShouldNot(HaveOccurred())

				secondServerConn = <-serverConnCh
			})

			AfterEach(func() {
				secondServerConn.Close()
				secondClientConn.Close()
			})

			JustBeforeEach(func() {
				err := fanout.Attach(secondServerConn)
				Ω(err).ShouldNot(HaveOccurred())
			})

			Describe("writing messages", func() {
				It("fans them out to anyone attached", func() {
					err := fanout.WriteMessage(rawMSG(`{"hello":1}`))
					Ω(err).ShouldNot(HaveOccurred())

					var msg *json.RawMessage
					err = clientConn.ReadJSON(&msg)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(msg).Should(Equal(rawMSG(`{"hello":1}`)))

					err = secondClientConn.ReadJSON(&msg)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(msg).Should(Equal(rawMSG(`{"hello":1}`)))
				})
			})
		})
	})
})
