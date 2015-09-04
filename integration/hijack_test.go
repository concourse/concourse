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
				Ω(err).ShouldNot(HaveOccurred())

				Ω(processSpec.User).Should(Equal("root"))

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

		commandWithArgs := append([]string{command}, args...)

		flyCmd := exec.Command(flyPath, append([]string{"-t", atcServer.URL()}, commandWithArgs...)...)
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
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/containers", "build-id=3&name=build"),
					ghttp.RespondWithJSONEncoded(200, atc.ListContainersReturn{
						Containers: []atc.PresentedContainer{
							{ID: "container-id-1", PipelineName: "pipeline-name-1", Type: "task", Name: "build", BuildID: 3},
						},
						Errors: nil,
					}),
				),
				hijackHandler("container-id-1", didHijack, nil),
			)
		})

		It("hijacks the most recent one-off build", func() {
			hijack()
		})

		It("hijacks the most recent one-off build with a more politically correct command", func() {
			fly("intercept")
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
					ghttp.VerifyRequest("GET", "/api/v1/containers", "build-id=1&name=build"),
					ghttp.RespondWithJSONEncoded(200, atc.ListContainersReturn{
						Containers: []atc.PresentedContainer{},
						Errors:     nil,
					}),
				),
				hijackHandler("container-id-1", didHijack, nil),
			)
		})

		It("return a friendly error message", func() {
			flyCmd := exec.Command(flyPath, "-t", atcServer.URL(), "hijack")
			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(1))

			Ω(sess.Err).Should(gbytes.Say("no containers matched your search parameters! they may have expired if your build hasn't recently finished"))
		})
	})

	Context("when multiple containers are found", func() {
		BeforeEach(func() {
			didHijack := make(chan struct{})
			hijacked = didHijack

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/containers", "type=check&name=some-resource-name"),
					ghttp.RespondWithJSONEncoded(200, atc.ListContainersReturn{
						Containers: []atc.PresentedContainer{
							{ID: "container-id-1", PipelineName: "pipeline-name-1", Type: "check", Name: "some-resource-name", BuildID: 6},
							{ID: "container-id-2", PipelineName: "pipeline-name-2", Type: "check", Name: "some-resource-name", BuildID: 5},
						},
						Errors: nil,
					}),
				),
				hijackHandler("container-id-2", didHijack, nil),
			)
		})

		It("asks the user to select the container from a menu", func() {
			pty, tty, err := pty.Open()
			Ω(err).ShouldNot(HaveOccurred())

			flyCmd := exec.Command(flyPath, "-t", atcServer.URL(), "hijack", "-c", "some-resource-name")
			flyCmd.Stdin = tty

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(sess.Out).Should(gbytes.Say("1. pipeline:pipeline-name-1, type:check, name:some-resource-name, build_id:6"))
			Eventually(sess.Out).Should(gbytes.Say("2. pipeline:pipeline-name-2, type:check, name:some-resource-name, build_id:5"))
			Eventually(sess.Out).Should(gbytes.Say("Choose a container: "))

			_, err = pty.WriteString("ghfdhf\n")
			Ω(err).ShouldNot(HaveOccurred())
			Eventually(sess.Out).Should(gbytes.Say("invalid selection"))
			Eventually(sess.Out).Should(gbytes.Say("Choose a container: "))

			_, err = pty.WriteString("3\n")
			Ω(err).ShouldNot(HaveOccurred())
			Eventually(sess.Out).Should(gbytes.Say("invalid selection"))
			Eventually(sess.Out).Should(gbytes.Say("Choose a container: "))

			_, err = pty.WriteString("2\n")
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
		})
	})

	Context("hijacks with check container", func() {
		BeforeEach(func() {
			didHijack := make(chan struct{})
			hijacked = didHijack

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/containers", "type=check&name=some-resource-name"),
					ghttp.RespondWithJSONEncoded(200, atc.ListContainersReturn{
						Containers: []atc.PresentedContainer{
							{ID: "container-id-1", PipelineName: "pipeline-name-1", Type: "check", Name: "some-resource-name", BuildID: 6},
						},
						Errors: nil,
					}),
				),
				hijackHandler("container-id-1", didHijack, nil),
			)
		})

		It("can accept the check resources name", func() {
			hijack("--check", "some-resource-name")
		})
	})

	Context("hijacks with check container with a pipeline", func() {
		BeforeEach(func() {
			didHijack := make(chan struct{})
			hijacked = didHijack

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/containers", "type=check&name=some-resource-name&pipeline=a-pipeline"),
					ghttp.RespondWithJSONEncoded(200, atc.ListContainersReturn{
						Containers: []atc.PresentedContainer{
							{ID: "container-id-1", PipelineName: "a-pipeline", Type: "check", Name: "some-resource-name", BuildID: 6},
						},
						Errors: nil,
					}),
				),
				hijackHandler("container-id-1", didHijack, nil),
			)
		})

		It("can accept the check resources name and a pipeline", func() {
			hijack("--check", "some-resource-name", "--pipeline", "a-pipeline")
		})
	})

	Context("if you only specify a pipeline", func() {
		It("returns an error", func() {
			flyCmd := exec.Command(flyPath, "-t", atcServer.URL(), "hijack", "-p", "pipeline-name")
			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(1))

			Ω(sess.Err).Should(gbytes.Say("job must be specified if pipeline is specified"))
		})
	})

	Context("with a specific build id", func() {
		BeforeEach(func() {
			didHijack := make(chan struct{})
			hijacked = didHijack

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/builds/2"),
					ghttp.RespondWithJSONEncoded(200, atc.Build{
						ID:     2,
						Name:   "1",
						Status: "started",
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/containers", "build-id=2&name=build"),
					ghttp.RespondWithJSONEncoded(200, atc.ListContainersReturn{
						Containers: []atc.PresentedContainer{
							{ID: "container-id-1", PipelineName: "a-pipeline", Type: "task", Name: "build", BuildID: 2},
						},
						Errors: nil,
					}),
				),
				hijackHandler("container-id-1", didHijack, nil),
			)
		})

		It("hijacks the most recent one-off build", func() {
			hijack("-b", "2")
		})
	})

	Context("with a specific job", func() {
		Context("when the job has a next build and pipeline", func() {
			BeforeEach(func() {
				didHijack := make(chan struct{})
				hijacked = didHijack

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
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/containers", "build-id=3&name=build"),
						ghttp.RespondWithJSONEncoded(200, atc.ListContainersReturn{
							Containers: []atc.PresentedContainer{
								{ID: "container-id-1", PipelineName: "some-pipeline", Type: "task", Name: "build", BuildID: 3},
							},
							Errors: nil,
						}),
					),
					hijackHandler("container-id-1", didHijack, nil),
				)
			})

			It("hijacks the job's next build", func() {
				hijack("--job", "some-job", "--pipeline", "some-pipeline")
			})
		})

		Context("when the job only has a finished build", func() {
			BeforeEach(func() {
				didHijack := make(chan struct{})
				hijacked = didHijack

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
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/containers", "build-id=3&name=build"),
						ghttp.RespondWithJSONEncoded(200, atc.ListContainersReturn{
							Containers: []atc.PresentedContainer{
								{ID: "container-id-1", PipelineName: "a-pipeline", Type: "task", Name: "build", BuildID: 3},
							},
							Errors: nil,
						}),
					),
					hijackHandler("container-id-1", didHijack, nil),
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
						ghttp.VerifyRequest("GET", "/api/v1/pipelines/main/jobs/some-job/builds/3"),
						ghttp.RespondWithJSONEncoded(200, atc.Build{
							ID:      3,
							Name:    "3",
							Status:  "failed",
							JobName: "some-job",
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/containers", "build-id=3&name=build"),
						ghttp.RespondWithJSONEncoded(200, atc.ListContainersReturn{
							Containers: []atc.PresentedContainer{
								{ID: "container-id-1", PipelineName: "a-pipeline", Type: "task", Name: "build", BuildID: 3},
							},
							Errors: nil,
						}),
					),
					hijackHandler("container-id-1", didHijack, nil),
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
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/containers", "build-id=3&name=build"),
						ghttp.RespondWithJSONEncoded(200, atc.ListContainersReturn{
							Containers: []atc.PresentedContainer{
								{ID: "container-id-1", PipelineName: "a-pipeline", Type: "task", Name: "build", BuildID: 3},
							},
							Errors: nil,
						}),
					),
					hijackHandler("container-id-1", didHijack, []string{"something went wrong"}),
				)
			})

			It("prints it to stderr and exits 255", func() {
				pty, tty, err := pty.Open()
				Ω(err).ShouldNot(HaveOccurred())

				flyCmd := exec.Command(flyPath, "-t", atcServer.URL(), "hijack")
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
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/containers", "build-id=3&type=get&name=money"),
					ghttp.RespondWithJSONEncoded(200, atc.ListContainersReturn{
						Containers: []atc.PresentedContainer{
							{ID: "container-id-1", PipelineName: "a-pipeline", Type: "get", Name: "money", BuildID: 3},
						},
						Errors: nil,
					}),
				),
				hijackHandler("container-id-1", didHijack, nil),
			)
		})

		It("hijacks the given type and name", func() {
			hijack("-t", "get", "-n", "money")
		})
	})

	Context("when a step type 'check' is specified", func() {
		BeforeEach(func() {
			didHijack := make(chan struct{})
			hijacked = didHijack

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/containers", "type=check"),
					ghttp.RespondWithJSONEncoded(200, atc.ListContainersReturn{
						Containers: []atc.PresentedContainer{
							{ID: "container-id-1", PipelineName: "a-pipeline", Type: "check", Name: "sum", BuildID: 3},
						},
						Errors: nil,
					}),
				),
				hijackHandler("container-id-1", didHijack, nil),
			)
		})

		It("should not consult the /builds endpoint", func() {
			hijack("-t", "check")
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
			Ω(err).ShouldNot(HaveOccurred())

			flyCmd := exec.Command(flyPath, "-t", atcServer.URL(), "hijack", "-b", "0")
			flyCmd.Stdin = tty

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(sess.Err.Contents).Should(ContainSubstring("bad response when getting build"))
			Eventually(sess.Err.Contents).Should(ContainSubstring("404 Not Found"))

			err = pty.Close()
			Ω(err).ShouldNot(HaveOccurred())

			<-sess.Exited
			Ω(sess.ExitCode()).Should(Equal(1))
		})
	})
})
