// +build !windows

package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
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

	})

	AfterEach(func() {
		atcServer.Close()
	})

	hijackHandler := func(id string, didHijack chan<- struct{}, errorMessages []string) http.HandlerFunc {
		return ghttp.CombineHandlers(
			ghttp.VerifyRequest("POST", fmt.Sprintf("/api/v1/containers/%s/hijack", id)),
			func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()

				w.WriteHeader(http.StatusOK)

				body := json.NewDecoder(r.Body)
				defer r.Body.Close()

				var processSpec atc.HijackProcessSpec
				err := body.Decode(&processSpec)
				Expect(err).NotTo(HaveOccurred())

				Expect(processSpec.User).To(Equal("root"))

				sconn, sbr, err := w.(http.Hijacker).Hijack()
				Expect(err).NotTo(HaveOccurred())

				defer sconn.Close()

				close(didHijack)

				decoder := json.NewDecoder(sbr)
				encoder := json.NewEncoder(sconn)

				var payload atc.HijackInput

				err = decoder.Decode(&payload)
				Expect(err).NotTo(HaveOccurred())

				Expect(payload).To(Equal(atc.HijackInput{
					Stdin: []byte("some stdin"),
				}))

				err = encoder.Encode(atc.HijackOutput{
					Stdout: []byte("some stdout"),
				})
				Expect(err).NotTo(HaveOccurred())

				err = encoder.Encode(atc.HijackOutput{
					Stderr: []byte("some stderr"),
				})
				Expect(err).NotTo(HaveOccurred())

				if len(errorMessages) > 0 {
					for _, msg := range errorMessages {
						err := encoder.Encode(atc.HijackOutput{
							Error: msg,
						})
						Expect(err).NotTo(HaveOccurred())
					}

					return
				}

				exitStatus := 123
				err = encoder.Encode(atc.HijackOutput{
					ExitStatus: &exitStatus,
				})
				Expect(err).NotTo(HaveOccurred())
			},
		)
	}

	fly := func(command string, args ...string) {
		pty, tty, err := pty.Open()
		Expect(err).NotTo(HaveOccurred())

		commandWithArgs := append([]string{command}, args...)

		flyCmd := exec.Command(flyPath, append([]string{"-t", atcServer.URL()}, commandWithArgs...)...)
		flyCmd.Stdin = tty

		sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(hijacked).Should(BeClosed())

		_, err = pty.WriteString("some stdin")
		Expect(err).NotTo(HaveOccurred())

		Eventually(sess.Out).Should(gbytes.Say("some stdout"))
		Eventually(sess.Err).Should(gbytes.Say("some stderr"))

		err = pty.Close()
		Expect(err).NotTo(HaveOccurred())

		<-sess.Exited
		Expect(sess.ExitCode()).To(Equal(123))
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
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/containers", "build-id=3&name=some-step"),
					ghttp.RespondWithJSONEncoded(200, []atc.Container{
						{ID: "container-id-1", PipelineName: "pipeline-name-1", Type: "task", Name: "some-step", BuildID: 3},
					}),
				),
				hijackHandler("container-id-1", didHijack, nil),
			)
		})

		It("hijacks the most recent one-off build", func() {
			hijack("-s", "some-step")
		})

		It("hijacks the most recent one-off build with a more politically correct command", func() {
			fly("intercept", "-s", "some-step")
		})
	})

	Context("when no containers are found", func() {
		BeforeEach(func() {
			didHijack := make(chan struct{})
			hijacked = didHijack

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/builds"),
					ghttp.RespondWithJSONEncoded(200, []atc.Build{
						{ID: 1, Name: "1", Status: "finished"},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/containers", "build-id=1&name=some-step"),
					ghttp.RespondWithJSONEncoded(200, []atc.Container{}),
				),
				hijackHandler("container-id-1", didHijack, nil),
			)
		})

		It("return a friendly error message", func() {
			flyCmd := exec.Command(flyPath, "-t", atcServer.URL(), "hijack", "-s", "some-step")
			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(1))

			Expect(sess.Err).To(gbytes.Say("no containers matched your search parameters! they may have expired if your build hasn't recently finished"))
		})
	})

	Context("when no builds are found", func() {
		BeforeEach(func() {
			didHijack := make(chan struct{})
			hijacked = didHijack

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/builds/0"),
					ghttp.RespondWithJSONEncoded(404, ""),
				),
			)
		})

		It("logs an error message and response status/body", func() {
			pty, tty, err := pty.Open()
			Expect(err).NotTo(HaveOccurred())

			flyCmd := exec.Command(flyPath, "-t", atcServer.URL(), "hijack", "-b", "0")
			flyCmd.Stdin = tty

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess.Err.Contents).Should(ContainSubstring("failed to get build"))

			err = pty.Close()
			Expect(err).NotTo(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(1))
		})
	})

	Context("when multiple containers are found", func() {
		BeforeEach(func() {
			didHijack := make(chan struct{})
			hijacked = didHijack

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/pipelines/pipeline-name-1/jobs/some-job"),
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
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/containers", "build-id=3&name="),
					ghttp.RespondWithJSONEncoded(200, []atc.Container{
						{ID: "container-id-1", PipelineName: "pipeline-name-1", Type: "get", Name: "some-job", BuildID: 3},
						{ID: "container-id-2", PipelineName: "pipeline-name-1", Type: "put", Name: "some-job", BuildID: 3},
					}),
				),
				hijackHandler("container-id-2", didHijack, nil),
			)
		})

		It("asks the user to select the container from a menu", func() {
			pty, tty, err := pty.Open()
			Expect(err).NotTo(HaveOccurred())

			flyCmd := exec.Command(flyPath, "-t", atcServer.URL(), "hijack", "-j", "pipeline-name-1/some-job")
			flyCmd.Stdin = tty

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess.Out).Should(gbytes.Say("1. pipeline: pipeline-name-1, build id: 3, type: get, name: some-job"))
			Eventually(sess.Out).Should(gbytes.Say("2. pipeline: pipeline-name-1, build id: 3, type: put, name: some-job"))
			Eventually(sess.Out).Should(gbytes.Say("choose a container: "))

			_, err = pty.WriteString("ghfdhf\n")
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess.Out).Should(gbytes.Say("invalid selection"))
			Eventually(sess.Out).Should(gbytes.Say("choose a container: "))

			_, err = pty.WriteString("3\n")
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess.Out).Should(gbytes.Say("invalid selection"))
			Eventually(sess.Out).Should(gbytes.Say("choose a container: "))

			_, err = pty.WriteString("2\n")
			Expect(err).NotTo(HaveOccurred())

			Eventually(hijacked).Should(BeClosed())

			_, err = pty.WriteString("some stdin")
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess.Out).Should(gbytes.Say("some stdout"))
			Eventually(sess.Err).Should(gbytes.Say("some stderr"))

			err = pty.Close()
			Expect(err).NotTo(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(123))
		})

		It("exits when the user ends the input stream (Ctrl+D)", func() {
			pty, tty, err := pty.Open()
			Expect(err).NotTo(HaveOccurred())

			flyCmd := exec.Command(flyPath, "-t", atcServer.URL(), "hijack", "-j", "pipeline-name-1/some-job")
			flyCmd.Stdin = tty

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess.Out).Should(gbytes.Say("1. pipeline: pipeline-name-1, build id: 3, type: get, name: some-job"))
			Eventually(sess.Out).Should(gbytes.Say("2. pipeline: pipeline-name-1, build id: 3, type: put, name: some-job"))
			Eventually(sess.Out).Should(gbytes.Say("choose a container: "))

			pty.Close()

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))
		})
	})

	Context("when hijack returns a single container", func() {
		var (
			containerArguments string
			stepType           string
			stepName           string
			buildID            int
			hijackHandlerError []string
			pipelineName       string
		)

		BeforeEach(func() {
			hijackHandlerError = nil
			pipelineName = "a-pipeline"
		})

		JustBeforeEach(func() {
			didHijack := make(chan struct{})
			hijacked = didHijack

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/containers", containerArguments),
					ghttp.RespondWithJSONEncoded(200, []atc.Container{
						{ID: "container-id-1", PipelineName: pipelineName, Type: stepType, Name: stepName, BuildID: buildID},
					}),
				),
				hijackHandler("container-id-1", didHijack, hijackHandlerError),
			)
		})

		Context("when called with check container", func() {
			BeforeEach(func() {
				stepType = "check"
				stepName = "some-resource-name"
				buildID = 6
			})

			Context("and no other arguments", func() {
				BeforeEach(func() {
					pipelineName = "main"
					containerArguments = "type=check&name=some-resource-name&pipeline=main"
				})

				It("can accept the check resources name", func() {
					hijack("--check", "some-resource-name")
				})
			})

			Context("and with pipeline specified", func() {
				BeforeEach(func() {
					containerArguments = "type=check&name=some-resource-name&pipeline=a-pipeline"
				})

				It("can accept the check resources name and a pipeline", func() {
					hijack("--check", "a-pipeline/some-resource-name")
				})
			})
		})

		Context("when called with a specific build id", func() {

			BeforeEach(func() {
				containerArguments = "build-id=2&name=some-step"
				stepType = "task"
				stepName = "some-step"
				buildID = 2

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/builds/2"),
						ghttp.RespondWithJSONEncoded(200, atc.Build{
							ID:     2,
							Name:   "1",
							Status: "started",
						}),
					),
				)
			})

			It("hijacks the most recent one-off build", func() {
				hijack("-b", "2", "-s", "some-step")
			})
		})

		Context("when called with a specific job", func() {
			BeforeEach(func() {
				containerArguments = "build-id=3&name=some-step"
				stepType = "task"
				stepName = "some-step"
				buildID = 3
			})

			Context("when the job has a next build and pipeline", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/pipelines/some-pipeline/jobs/some-job"),
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
					)
				})

				It("hijacks the job's next build", func() {
					hijack("--job", "some-pipeline/some-job", "--step", "some-step")
				})
			})

			Context("when the job only has a finished build", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/pipelines/main/jobs/some-job"),
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
					)
				})

				It("hijacks the job's finished build", func() {
					hijack("--job", "some-job", "--step", "some-step")
				})
			})

			Context("with a specific build of the job", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/pipelines/main/jobs/some-job/builds/3"),
							ghttp.RespondWithJSONEncoded(200, atc.Build{
								ID:      3,
								Name:    "3",
								Status:  "failed",
								JobName: "some-job",
							}),
						),
					)
				})

				It("hijacks the given build", func() {
					hijack("--job", "some-job", "--build", "3", "--step", "some-step")
				})
			})

			Context("when hijacking yields an error", func() {
				BeforeEach(func() {
					hijackHandlerError = []string{"something went wrong"}

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
					)
				})

				It("prints it to stderr and exits 255", func() {
					pty, tty, err := pty.Open()
					Expect(err).NotTo(HaveOccurred())

					flyCmd := exec.Command(flyPath, "-t", atcServer.URL(), "hijack", "--step", "some-step")
					flyCmd.Stdin = tty

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(hijacked).Should(BeClosed())

					_, err = pty.WriteString("some stdin")
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Err.Contents).Should(ContainSubstring(ansi.Color("something went wrong", "red+b") + "\n"))

					err = pty.Close()
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(255))
				})
			})
		})

		Context("when called with a step name specified", func() {
			BeforeEach(func() {
				containerArguments = "build-id=3&name=money"
				stepName = "money"
				buildID = 3

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
				)
			})

			It("hijacks the given type and name", func() {
				hijack("-s", "money")
			})
		})
	})
})
