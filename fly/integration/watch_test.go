package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/vito/go-sse/sse"

	"github.com/concourse/atc"
	"github.com/concourse/atc/event"
)

var _ = Describe("Watching", func() {
	var streaming chan struct{}
	var events chan atc.Event

	BeforeEach(func() {
		streaming = make(chan struct{})
		events = make(chan atc.Event)
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

	watch := func(args ...string) {
		watchWithArgs := append([]string{"watch"}, args...)

		flyCmd := exec.Command(flyPath, append([]string{"-t", targetName}, watchWithArgs...)...)

		sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(streaming).Should(BeClosed())

		events <- event.Log{Payload: "sup"}

		Eventually(sess.Out).Should(gbytes.Say("sup"))

		close(events)

		<-sess.Exited
		Expect(sess.ExitCode()).To(Equal(0))
	}

	Context("with no arguments", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/builds"),
					ghttp.RespondWithJSONEncoded(200, []atc.Build{
						{ID: 4, Name: "1", Status: "started", JobName: "some-job"},
						{ID: 3, Name: "3", Status: "started"},
						{ID: 2, Name: "2", Status: "started"},
						{ID: 1, Name: "1", Status: "finished"},
					}),
				),
				eventsHandler(),
			)
		})

		It("watches the most recent one-off build", func() {
			watch()
		})
	})

	Context("with a build ID and no job", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				eventsHandler(),
			)
		})

		It("Watches the given build id", func() {
			watch("--build", "3")
		})
	})

	Context("with a specific job and pipeline", func() {
		Context("when the job has no builds", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/some-pipeline/jobs/some-job"),
						ghttp.RespondWithJSONEncoded(200, atc.Job{}),
					),
					eventsHandler(),
				)
			})

			It("returns an error and exits", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "watch", "--job", "some-pipeline/some-job")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Err).Should(gbytes.Say("job has no builds"))
				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
			})
		})

		Context("when the job has a next build", func() {
			BeforeEach(func() {
				didStream := make(chan struct{})
				streaming = didStream

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/some-pipeline/jobs/some-job"),
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
				watch("--job", "some-pipeline/some-job")
			})
		})

		Context("when the job only has a finished build", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/main/jobs/some-job"),
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
				watch("--job", "main/some-job")
			})
		})

		Context("with a specific build of the job", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/main/jobs/some-job/builds/3"),
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
				watch("--job", "main/some-job", "--build", "3")
			})
		})
	})
})
