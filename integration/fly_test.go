package integration_test

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/vito/go-sse/sse"

	"github.com/concourse/atc"
	"github.com/concourse/atc/event"
)

var _ = Describe("Fly CLI", func() {
	var flyPath string
	var buildDir string

	var atcServer *ghttp.Server
	var streaming chan struct{}
	var events chan atc.Event
	var uploadingBits <-chan struct{}

	var expectedPlan atc.Plan

	BeforeEach(func() {
		var err error

		flyPath, err = gexec.Build("github.com/concourse/fly")
		Ω(err).ShouldNot(HaveOccurred())

		buildDir, err = ioutil.TempDir("", "fly-build-dir")
		Ω(err).ShouldNot(HaveOccurred())

		err = ioutil.WriteFile(
			filepath.Join(buildDir, "build.yml"),
			[]byte(`---
platform: some-platform

image: ubuntu

params:
  FOO: bar
  BAZ: buzz
  X: 1

run:
  path: find
  args: [.]
`),
			0644,
		)
		Ω(err).ShouldNot(HaveOccurred())

		atcServer = ghttp.NewServer()

		os.Setenv("ATC_URL", atcServer.URL())

		streaming = make(chan struct{})
		events = make(chan atc.Event)

		expectedPlan = atc.Plan{
			Compose: &atc.ComposePlan{
				A: atc.Plan{
					Aggregate: &atc.AggregatePlan{
						filepath.Base(buildDir): atc.Plan{
							Get: &atc.GetPlan{
								Name: filepath.Base(buildDir),
								Type: "archive",
								Source: atc.Source{
									"uri": "http://127.0.0.1:1234/api/v1/pipes/some-pipe-id",
								},
							},
						},
					},
				},
				B: atc.Plan{
					Task: &atc.TaskPlan{
						Name: "build",
						Config: &atc.TaskConfig{
							Platform: "some-platform",
							Image:    "ubuntu",
							Params: map[string]string{
								"FOO": "bar",
								"BAZ": "buzz",
								"X":   "1",
							},
							Run: atc.TaskRunConfig{
								Path: "find",
								Args: []string{"."},
							},
						},
					},
				},
			},
		}
	})

	JustBeforeEach(func() {
		uploading := make(chan struct{})
		uploadingBits = uploading

		atcServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/api/v1/pipes"),
				ghttp.RespondWithJSONEncoded(http.StatusCreated, atc.Pipe{
					ID:       "some-pipe-id",
					PeerAddr: "127.0.0.1:1234",
				}),
			),
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/api/v1/builds"),
				ghttp.VerifyJSONRepresenting(expectedPlan),
				func(w http.ResponseWriter, r *http.Request) {
					http.SetCookie(w, &http.Cookie{
						Name:    "Some-Cookie",
						Value:   "some-cookie-data",
						Path:    "/",
						Expires: time.Now().Add(1 * time.Minute),
					})
				},
				ghttp.RespondWith(201, `{"id":128}`),
			),
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/api/v1/builds/128/events"),
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
						payload, err := json.Marshal(event.Message{e})
						Ω(err).ShouldNot(HaveOccurred())

						event := sse.Event{
							ID:   fmt.Sprintf("%d", id),
							Name: "event",
							Data: payload,
						}

						err = event.Write(w)
						Ω(err).ShouldNot(HaveOccurred())

						flusher.Flush()

						id++
					}

					err := sse.Event{
						Name: "end",
					}.Write(w)
					Ω(err).ShouldNot(HaveOccurred())
				},
			),
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("PUT", "/api/v1/pipes/some-pipe-id"),
				func(w http.ResponseWriter, req *http.Request) {
					close(uploading)

					gr, err := gzip.NewReader(req.Body)
					Ω(err).ShouldNot(HaveOccurred())

					tr := tar.NewReader(gr)

					hdr, err := tr.Next()
					Ω(err).ShouldNot(HaveOccurred())

					Ω(hdr.Name).Should(Equal("./"))

					hdr, err = tr.Next()
					Ω(err).ShouldNot(HaveOccurred())

					Ω(hdr.Name).Should(MatchRegexp("(./)?build.yml$"))
				},
				ghttp.RespondWith(200, ""),
			),
		)
	})

	It("creates a build, streams output, uploads the bits, and polls until completion", func() {
		flyCmd := exec.Command(flyPath)
		flyCmd.Dir = buildDir

		sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		Eventually(streaming).Should(BeClosed())

		events <- event.Log{Payload: "sup"}

		Eventually(sess.Out).Should(gbytes.Say("sup"))

		close(events)

		<-sess.Exited
		Ω(sess.ExitCode()).Should(Equal(0))
	})

	Context("when the build config is invalid", func() {
		BeforeEach(func() {
			// missing platform and run path
			err := ioutil.WriteFile(
				filepath.Join(buildDir, "build.yml"),
				[]byte(`---
run: {}
`),
				0644,
			)
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("prints the failure and exits 1", func() {
			flyCmd := exec.Command(flyPath)
			flyCmd.Dir = buildDir

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(sess.Err).Should(gbytes.Say("missing"))

			<-sess.Exited
			Ω(sess.ExitCode()).Should(Equal(1))
		})
	})

	Context("when arguments are passed through", func() {
		BeforeEach(func() {
			expectedPlan.Compose.B.Task.Config.Run.Args = []string{".", "-name", `foo "bar" baz`}
		})

		It("inserts them into the config template", func() {
			atcServer.AllowUnhandledRequests = true

			flyCmd := exec.Command(flyPath, "--", "-name", "foo \"bar\" baz")
			flyCmd.Dir = buildDir

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())

			// sync with after create
			Eventually(streaming, 5.0).Should(BeClosed())

			close(events)

			<-sess.Exited
			Ω(sess.ExitCode()).Should(Equal(0))
		})
	})

	Context("when running with --privileged", func() {
		BeforeEach(func() {
			expectedPlan.Compose.B.Task.Privileged = true
		})

		It("inserts them into the config template", func() {
			atcServer.AllowUnhandledRequests = true

			flyCmd := exec.Command(flyPath, "--privileged")
			flyCmd.Dir = buildDir

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())

			// sync with after create
			Eventually(streaming, 5.0).Should(BeClosed())

			close(events)

			<-sess.Exited
			Ω(sess.ExitCode()).Should(Equal(0))
		})
	})

	Context("when running with bogus flags", func() {
		It("exits 1", func() {
			flyCmd := exec.Command(flyPath, "--bogus-flag")
			flyCmd.Dir = buildDir

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(sess).Should(gbytes.Say("Incorrect Usage."))
			Eventually(sess.Err).Should(gbytes.Say("bogus-flag"))

			<-sess.Exited
			Ω(sess.ExitCode()).Should(Equal(1))
		})
	})

	Context("when parameters are specified in the environment", func() {
		BeforeEach(func() {
			expectedPlan.Compose.B.Task.Config.Params = map[string]string{
				"FOO": "newbar",
				"BAZ": "buzz",
				"X":   "",
			}
		})

		It("overrides the build's paramter values", func() {
			atcServer.AllowUnhandledRequests = true

			flyCmd := exec.Command(flyPath)
			flyCmd.Dir = buildDir
			flyCmd.Env = append(os.Environ(), "FOO=newbar", "X=")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())

			// sync with after create
			Eventually(streaming, 5.0).Should(BeClosed())

			close(events)

			<-sess.Exited
			Ω(sess.ExitCode()).Should(Equal(0))
		})
	})

	Context("when the build is interrupted", func() {
		var aborted chan struct{}

		JustBeforeEach(func() {
			aborted = make(chan struct{})

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/api/v1/builds/128/abort"),
					func(w http.ResponseWriter, r *http.Request) {
						close(aborted)
					},
				),
			)
		})

		if runtime.GOOS != "windows" {
			Describe("with SIGINT", func() {
				It("aborts the build and exits nonzero", func() {
					flyCmd := exec.Command(flyPath)
					flyCmd.Dir = buildDir

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).ToNot(HaveOccurred())

					Eventually(streaming, 5).Should(BeClosed())

					Eventually(uploadingBits).Should(BeClosed())

					sess.Signal(os.Interrupt)

					Eventually(aborted, 5.0).Should(BeClosed())

					events <- event.Status{Status: atc.StatusErrored}
					close(events)

					<-sess.Exited
					Ω(sess.ExitCode()).Should(Equal(2))
				})
			})

			Describe("with SIGTERM", func() {
				It("aborts the build and exits nonzero", func() {
					flyCmd := exec.Command(flyPath)
					flyCmd.Dir = buildDir

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).ToNot(HaveOccurred())

					Eventually(streaming, 5).Should(BeClosed())

					Eventually(uploadingBits).Should(BeClosed())

					sess.Signal(syscall.SIGTERM)

					Eventually(aborted, 5.0).Should(BeClosed())

					events <- event.Status{Status: atc.StatusErrored}
					close(events)

					<-sess.Exited
					Ω(sess.ExitCode()).Should(Equal(2))
				})
			})
		}
	})

	Context("when the build succeeds", func() {
		It("exits 0", func() {
			flyCmd := exec.Command(flyPath)
			flyCmd.Dir = buildDir

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(streaming, 5).Should(BeClosed())

			events <- event.Status{Status: atc.StatusSucceeded}
			close(events)

			<-sess.Exited
			Ω(sess.ExitCode()).Should(Equal(0))
		})
	})

	Context("when the build fails", func() {
		It("exits 1", func() {
			flyCmd := exec.Command(flyPath)
			flyCmd.Dir = buildDir

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(streaming, 5).Should(BeClosed())

			events <- event.Status{Status: atc.StatusFailed}
			close(events)

			<-sess.Exited
			Ω(sess.ExitCode()).Should(Equal(1))
		})
	})

	Context("when the build errors", func() {
		It("exits 2", func() {
			flyCmd := exec.Command(flyPath)
			flyCmd.Dir = buildDir

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(streaming, 5).Should(BeClosed())

			events <- event.Status{Status: atc.StatusErrored}
			close(events)

			<-sess.Exited
			Ω(sess.ExitCode()).Should(Equal(2))
		})
	})
})
