package concourse_test

import (
	"fmt"
	"io"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/stream"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/go-concourse/concourse"
	"github.com/concourse/concourse/go-concourse/concourse/concoursefakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Events", func() {
	Describe("Events", func() {
		buildID := "3"

		var eventsChan chan atc.Event
		var visitor *concoursefakes.FakeBuildEventsVisitor

		BeforeEach(func() {
			eventsChan = make(chan atc.Event, 2)
			eventsChan <- event.Status{Status: atc.StatusStarted}
			eventsChan <- event.Status{Status: atc.StatusSucceeded}
			close(eventsChan)

			visitor = new(concoursefakes.FakeBuildEventsVisitor)
		})

		eventsHandler := func() http.HandlerFunc {
			return ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", fmt.Sprintf("/api/v1/builds/%s/events", buildID)),
				func(w http.ResponseWriter, r *http.Request) {
					stream.WriteHeaders(w)
					writer := stream.EventWriter{w.(stream.WriteFlusher)}

					id := uint(0)
					for e := range eventsChan {
						err := writer.WriteEvent(id, "event", event.Message{Event: e})
						Expect(err).NotTo(HaveOccurred())
						id++
					}

					err := writer.WriteEnd(id)
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

				err = stream.Accept(visitor)
				Expect(err).NotTo(HaveOccurred())
				Expect(visitor.VisitEventArgsForCall(0)).To(Equal(event.Status{
					Status: atc.StatusStarted,
				}))

				err = stream.Accept(visitor)
				Expect(err).NotTo(HaveOccurred())
				Expect(visitor.VisitEventArgsForCall(1)).To(Equal(event.Status{
					Status: atc.StatusSucceeded,
				}))

				err = stream.Accept(visitor)
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
