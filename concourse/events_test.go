package concourse_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/event"
	"github.com/concourse/go-concourse/concourse"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/vito/go-sse/sse"
)

var _ = Describe("ATC Handler Events", func() {
	Describe("Events", func() {
		buildID := "3"

		var streaming chan struct{}
		var eventsChan chan atc.Event

		BeforeEach(func() {
			streaming = make(chan struct{})

			eventsChan = make(chan atc.Event, 2)
			eventsChan <- event.Status{Status: atc.StatusStarted}
			eventsChan <- event.Status{Status: atc.StatusSucceeded}
			close(eventsChan)
		})

		eventsHandler := func() http.HandlerFunc {
			return ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", fmt.Sprintf("/api/v1/builds/%s/events", buildID)),
				func(w http.ResponseWriter, r *http.Request) {
					flusher := w.(http.Flusher)

					w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
					w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
					w.Header().Add("Connection", "keep-alive")

					w.WriteHeader(http.StatusOK)

					flusher.Flush()

					close(streaming)

					id := 0

					for e := range eventsChan {
						payload, err := json.Marshal(event.Message{Event: e})
						Expect(err).NotTo(HaveOccurred())

						event := sse.Event{
							ID:   fmt.Sprintf("%d", id),
							Name: "event",
							Data: payload,
						}

						err = event.Write(w)
						Expect(err).NotTo(HaveOccurred())

						flusher.Flush()

						id++
					}

					err := sse.Event{
						Name: "end",
					}.Write(w)
					Expect(err).NotTo(HaveOccurred())
				},
			)
		}

		Context("when the server returns events", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					eventsHandler(),
				)
			})

			It("returns events that can stream events", func() {
				stream, err := client.BuildEvents(buildID)
				Expect(err).NotTo(HaveOccurred())

				next, err := stream.NextEvent()
				Expect(err).NotTo(HaveOccurred())
				Expect(next).To(Equal(event.Status{
					Status: atc.StatusStarted,
				}))

				next, err = stream.NextEvent()
				Expect(err).NotTo(HaveOccurred())
				Expect(next).To(Equal(event.Status{
					Status: atc.StatusSucceeded,
				}))

				_, err = stream.NextEvent()
				Expect(err).To(Equal(io.EOF))

				err = stream.Close()
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when the server returns 401", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(ghttp.RespondWith(http.StatusUnauthorized, ""))
			})

			It("returns ErrUnauthorized", func() {
				_, err := client.BuildEvents(buildID)
				Expect(err).To(Equal(concourse.ErrUnauthorized))
			})
		})

		Context("when the server returns 403", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(ghttp.RespondWith(http.StatusForbidden, ""))
			})

			It("returns ErrForbidden", func() {
				_, err := client.BuildEvents(buildID)
				Expect(err).To(Equal(concourse.ErrForbidden))
			})
		})
	})
})
