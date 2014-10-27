package integration_test

import (
	"encoding/gob"
	"net/http"
	"os"
	"os/exec"

	"github.com/concourse/atc"
	"github.com/concourse/turbine"
	"github.com/kr/pty"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Hijacking", func() {
	var atcServer *ghttp.Server
	var hijacked <-chan struct{}

	BeforeEach(func() {
		atcServer = ghttp.NewServer()
		hijacked = nil

		os.Setenv("ATC_URL", atcServer.URL())
	})

	hijackHandler := func(didHijack chan<- struct{}) http.HandlerFunc {
		return ghttp.CombineHandlers(
			ghttp.VerifyRequest("POST", "/api/v1/builds/3/hijack"),
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)

				sconn, sbr, err := w.(http.Hijacker).Hijack()
				Ω(err).ShouldNot(HaveOccurred())

				defer sconn.Close()

				close(didHijack)

				decoder := gob.NewDecoder(sbr)

				var payload turbine.HijackPayload

				err = decoder.Decode(&payload)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(payload).Should(Equal(turbine.HijackPayload{
					Stdin: []byte("marco"),
				}))

				_, err = sconn.Write([]byte("polo"))
				Ω(err).ShouldNot(HaveOccurred())
			},
		)
	}

	hijack := func(args ...string) {
		pty, tty, err := pty.Open()
		Ω(err).ShouldNot(HaveOccurred())

		flyCmd := exec.Command(flyPath, append([]string{"hijack"}, args...)...)
		flyCmd.Stdin = tty

		sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		Eventually(hijacked).Should(BeClosed())

		_, err = pty.WriteString("marco")
		Ω(err).ShouldNot(HaveOccurred())

		Eventually(sess).Should(gbytes.Say("polo"))

		err = pty.Close()
		Ω(err).ShouldNot(HaveOccurred())

		Eventually(sess).Should(gexec.Exit(0))
	}

	Context("with no arguments", func() {
		BeforeEach(func() {
			didHijack := make(chan struct{})
			hijacked = didHijack

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/builds"),
					ghttp.RespondWithJSONEncoded(200, []atc.Build{
						{ID: 3, Name: "3", Status: "started"},
						{ID: 2, Name: "2", Status: "started"},
						{ID: 1, Name: "1", Status: "finished"},
					}),
				),
				hijackHandler(didHijack),
			)
		})

		It("hijacks the most recent build", func() {
			hijack()
		})
	})

	Context("with a specific job", func() {
		Context("when the job has a next build", func() {
			BeforeEach(func() {
				didHijack := make(chan struct{})
				hijacked = didHijack

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
					hijackHandler(didHijack),
				)
			})

			It("hijacks the job's next build", func() {
				hijack("--job", "some-job")
			})
		})

		Context("when the job only has a finished build", func() {
			BeforeEach(func() {
				didHijack := make(chan struct{})
				hijacked = didHijack

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
					hijackHandler(didHijack),
				)
			})

			It("hijacks the job's finished build", func() {
				hijack("--job", "some-job")
			})
		})

		Context("with a specific build of the job", func() {
			BeforeEach(func() {
				didHijack := make(chan struct{})
				hijacked = didHijack

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
					hijackHandler(didHijack),
				)
			})

			It("hijacks the given build", func() {
				hijack("--job", "some-job", "--build", "3")
			})
		})
	})
})
