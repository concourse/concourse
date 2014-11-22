package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"

	"github.com/concourse/atc"
	"github.com/concourse/turbine/event"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/vito/go-sse/sse"
)

var _ = Describe("Watching", func() {
	var atcServer *ghttp.Server
	var streaming chan struct{}
	var events chan event.Event

	BeforeEach(func() {
		atcServer = ghttp.NewServer()
		streaming = make(chan struct{})
		events = make(chan event.Event)

		os.Setenv("ATC_URL", atcServer.URL())
	})

	eventsHandler := func() http.HandlerFunc {
		return ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", "/api/v1/builds/3/events"),
			func(w http.ResponseWriter, r *http.Request) {
				flusher := w.(http.Flusher)

				w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
				w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
				w.Header().Add("Connection", "keep-alive")

				w.WriteHeader(http.StatusOK)

				flusher.Flush()

				version := sse.Event{
					ID:   "0",
					Name: "version",
					Data: []byte(`"1.1"`),
				}

				err := version.Write(w)
				Ω(err).ShouldNot(HaveOccurred())

				flusher.Flush()

				close(streaming)

				id := 1

				for e := range events {
					payload, err := json.Marshal(e)
					Ω(err).ShouldNot(HaveOccurred())

					event := sse.Event{
						ID:   fmt.Sprintf("%d", id),
						Name: string(e.EventType()),
						Data: []byte(payload),
					}

					err = event.Write(w)
					Ω(err).ShouldNot(HaveOccurred())

					flusher.Flush()

					id++
				}
			},
		)
	}

	watch := func(args ...string) {
		flyCmd := exec.Command(flyPath, append([]string{"watch"}, args...)...)

		sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		Eventually(streaming).Should(BeClosed())

		events <- event.Log{Payload: "sup"}

		Eventually(sess.Out).Should(gbytes.Say("sup"))

		close(events)

		Eventually(sess).Should(gexec.Exit(0))
	}

	Context("with no arguments", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/builds"),
					ghttp.RespondWithJSONEncoded(200, []atc.Build{
						{ID: 3, Name: "3", Status: "started"},
						{ID: 2, Name: "2", Status: "started"},
						{ID: 1, Name: "1", Status: "finished"},
					}),
				),
				eventsHandler(),
			)
		})

		It("watches the most recent build", func() {
			watch()
		})
	})

	Context("with a specific job", func() {
		Context("when the job has a next build", func() {
			BeforeEach(func() {
				didStream := make(chan struct{})
				streaming = didStream

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/jobs/some-job"),
						ghttp.RespondWithJSONEncoded(200, atc.Job{
							NextBuild: &atc.Build{
								ID:      3,
								Name:    "3",
								Status:  "started",
								JobName: "some-job",
							},
							FinishedBuild: &atc.Build{
								ID:      2,
								Name:    "2",
								Status:  "failed",
								JobName: "some-job",
							},
						}),
					),
					eventsHandler(),
				)
			})

			It("watches the job's next build", func() {
				watch("--job", "some-job")
			})
		})

		Context("when the job only has a finished build", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/jobs/some-job"),
						ghttp.RespondWithJSONEncoded(200, atc.Job{
							NextBuild: nil,
							FinishedBuild: &atc.Build{
								ID:      3,
								Name:    "3",
								Status:  "failed",
								JobName: "some-job",
							},
						}),
					),
					eventsHandler(),
				)
			})

			It("watches the job's finished build", func() {
				watch("--job", "some-job")
			})
		})

		Context("with a specific build of the job", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/jobs/some-job/builds/3"),
						ghttp.RespondWithJSONEncoded(200, atc.Build{
							ID:      3,
							Name:    "3",
							Status:  "failed",
							JobName: "some-job",
						}),
					),
					eventsHandler(),
				)
			})

			It("watches the given build", func() {
				watch("--job", "some-job", "--build", "3")
			})
		})
	})
})
