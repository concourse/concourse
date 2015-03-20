// +build !windows

package integration_test

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"

	"github.com/concourse/atc"
	"github.com/kr/pty"
	"github.com/mgutz/ansi"
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

	hijackHandler := func(didHijack chan<- struct{}, rawQuery []string, errorMessages []string) http.HandlerFunc {
		return ghttp.CombineHandlers(
			ghttp.VerifyRequest("POST", "/api/v1/builds/3/hijack", rawQuery...),
			func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()

				w.WriteHeader(http.StatusOK)

				sconn, sbr, err := w.(http.Hijacker).Hijack()
				Ω(err).ShouldNot(HaveOccurred())

				defer sconn.Close()

				close(didHijack)

				decoder := json.NewDecoder(sbr)
				encoder := json.NewEncoder(sconn)

				var payload atc.HijackInput

				err = decoder.Decode(&payload)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(payload).Should(Equal(atc.HijackInput{
					Stdin: []byte("some stdin"),
				}))

				err = encoder.Encode(atc.HijackOutput{
					Stdout: []byte("some stdout"),
				})
				Ω(err).ShouldNot(HaveOccurred())

				err = encoder.Encode(atc.HijackOutput{
					Stderr: []byte("some stderr"),
				})
				Ω(err).ShouldNot(HaveOccurred())

				if len(errorMessages) > 0 {
					for _, msg := range errorMessages {
						err := encoder.Encode(atc.HijackOutput{
							Error: msg,
						})
						Ω(err).ShouldNot(HaveOccurred())
					}

					return
				}

				exitStatus := 123
				err = encoder.Encode(atc.HijackOutput{
					ExitStatus: &exitStatus,
				})
				Ω(err).ShouldNot(HaveOccurred())
			},
		)
	}

	fly := func(command string, args ...string) {
		pty, tty, err := pty.Open()
		Ω(err).ShouldNot(HaveOccurred())

		flyCmd := exec.Command(flyPath, append([]string{command}, args...)...)
		flyCmd.Stdin = tty

		sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		Eventually(hijacked).Should(BeClosed())

		_, err = pty.WriteString("some stdin")
		Ω(err).ShouldNot(HaveOccurred())

		Eventually(sess.Out).Should(gbytes.Say("some stdout"))
		Eventually(sess.Err).Should(gbytes.Say("some stderr"))

		err = pty.Close()
		Ω(err).ShouldNot(HaveOccurred())

		<-sess.Exited
		Ω(sess.ExitCode()).Should(Equal(123))
	}

	hijack := func(args ...string) {
		fly("hijack", args...)
	}

	Context("with no arguments", func() {
		BeforeEach(func() {
			didHijack := make(chan struct{})
			hijacked = didHijack

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
				hijackHandler(didHijack, []string{"type=task&name=build"}, nil),
			)
		})

		It("hijacks the most recent one-off build", func() {
			hijack()
		})

		It("hijacks the most recent one-off build with a more politically correct command", func() {
			fly("intercept")
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
					hijackHandler(didHijack, nil, nil),
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
					hijackHandler(didHijack, nil, nil),
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
					hijackHandler(didHijack, nil, nil),
				)
			})

			It("hijacks the given build", func() {
				hijack("--job", "some-job", "--build", "3")
			})
		})

		Context("when hijacking yields an error", func() {
			BeforeEach(func() {
				didHijack := make(chan struct{})
				hijacked = didHijack

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
					hijackHandler(didHijack, nil, []string{"something went wrong"}),
				)
			})

			It("prints it to stderr and exits 255", func() {
				pty, tty, err := pty.Open()
				Ω(err).ShouldNot(HaveOccurred())

				flyCmd := exec.Command(flyPath, "hijack")
				flyCmd.Stdin = tty

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Ω(err).ShouldNot(HaveOccurred())

				Eventually(hijacked).Should(BeClosed())

				_, err = pty.WriteString("some stdin")
				Ω(err).ShouldNot(HaveOccurred())

				Eventually(sess.Err.Contents).Should(ContainSubstring(ansi.Color("something went wrong", "red+b") + "\n"))

				err = pty.Close()
				Ω(err).ShouldNot(HaveOccurred())

				<-sess.Exited
				Ω(sess.ExitCode()).Should(Equal(255))
			})
		})
	})

	Context("when a step type and name are specified", func() {
		BeforeEach(func() {
			didHijack := make(chan struct{})
			hijacked = didHijack

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
				hijackHandler(didHijack, []string{"type=get&name=money"}, nil),
			)
		})

		It("hijacks the given type and name", func() {
			hijack("-t", "get", "-n", "money")
		})
	})
})
