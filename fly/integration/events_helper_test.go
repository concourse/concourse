package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/event"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/vito/go-sse/sse"
)

func BuildEventsHandler(buildID int, streaming chan<- struct{}, events <-chan atc.Event) http.HandlerFunc {
	return ghttp.CombineHandlers(
		ghttp.VerifyRequest("GET", fmt.Sprintf("/api/v1/builds/%d/events", buildID)),
		func(w http.ResponseWriter, r *http.Request) {
			flusher := w.(http.Flusher)

			w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
			w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Add("Connection", "keep-alive")

			w.WriteHeader(http.StatusOK)

			flusher.Flush()

			close(streaming)

			id := 0

			for e := range events {
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

func AssertEvents(sess *gexec.Session, streaming <-chan struct{}, events chan<- atc.Event) {
	Eventually(streaming).Should(BeClosed())

	events <- event.Log{Payload: "sup"}

	Eventually(sess.Out).Should(gbytes.Say("sup"))

	close(events)

	<-sess.Exited
	Expect(sess.ExitCode()).To(Equal(0))
}

func AssertErrorEvents(sess *gexec.Session, streaming <-chan struct{}, events chan<- atc.Event) {
	Eventually(streaming).Should(BeClosed())

	events <- event.Error{Message: "oh no"}
	events <- event.Status{Status: atc.StatusErrored}

	Eventually(sess.Out).Should(gbytes.Say("oh no"))

	close(events)

	<-sess.Exited
	Expect(sess.ExitCode()).To(Equal(2))
}
