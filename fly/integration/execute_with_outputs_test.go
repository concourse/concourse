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
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/event"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/vito/go-sse/sse"
)

var _ = Describe("Fly CLI", func() {
	var tmpdir string
	var buildDir string
	var taskConfigPath string

	var streaming chan struct{}
	var events chan atc.Event
	var outputDir string

	var expectedPlan atc.Plan
	var workerArtifact = atc.WorkerArtifact{
		ID:   125,
		Name: "some-dir",
	}

	BeforeEach(func() {
		var err error
		tmpdir, err = ioutil.TempDir("", "fly-build-dir")
		Expect(err).NotTo(HaveOccurred())

		outputDir, err = ioutil.TempDir("", "fly-task-output")
		Expect(err).NotTo(HaveOccurred())

		buildDir = filepath.Join(tmpdir, "fixture")

		err = os.Mkdir(buildDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		taskConfigPath = filepath.Join(buildDir, "task.yml")

		err = ioutil.WriteFile(
			taskConfigPath,
			[]byte(`---
platform: some-platform

image_resource:
  type: registry-image
  source:
    repository: ubuntu

inputs:
- name: fixture

outputs:
- name: some-dir

params:
  FOO: bar
  BAZ: buzz
  X: 1

run:
  path: /bin/sh
  args:
    - -c
    - echo some-content > some-dir/a-file

`),
			0644,
		)
		Expect(err).NotTo(HaveOccurred())

		streaming = make(chan struct{})
		events = make(chan atc.Event)

		planFactory := atc.NewPlanFactory(0)

		expectedPlan = planFactory.NewPlan(atc.EnsurePlan{
			Step: planFactory.NewPlan(atc.DoPlan{
				planFactory.NewPlan(atc.AggregatePlan{
					planFactory.NewPlan(atc.ArtifactInputPlan{
						Name:       filepath.Base(buildDir),
						ArtifactID: 125,
					}),
				}),
				planFactory.NewPlan(atc.TaskPlan{
					Name: "one-off",
					Config: &atc.TaskConfig{
						Platform: "some-platform",
						ImageResource: &atc.ImageResource{
							Type: "registry-image",
							Source: atc.Source{
								"repository": "ubuntu",
							},
						},
						Inputs: []atc.TaskInputConfig{
							{Name: "fixture"},
						},
						Outputs: []atc.TaskOutputConfig{
							{Name: "some-dir"},
						},
						Params: map[string]string{
							"FOO": "bar",
							"BAZ": "buzz",
							"X":   "1",
						},
						Run: atc.TaskRunConfig{
							Path: "/bin/sh",
							Args: []string{"-c", "echo some-content > some-dir/a-file"},
						},
					},
				}),
			}),
			Next: planFactory.NewPlan(atc.AggregatePlan{
				planFactory.NewPlan(atc.ArtifactOutputPlan{
					Name: "some-dir",
				}),
			}),
		})
	})

	AfterEach(func() {
		err := os.RemoveAll(tmpdir)
		Expect(err).NotTo(HaveOccurred())

		err = os.RemoveAll(outputDir)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		atcServer.RouteToHandler("POST", "/api/v1/teams/main/artifacts",
			ghttp.CombineHandlers(
				func(w http.ResponseWriter, req *http.Request) {
					Expect(req.FormValue("platform")).To(Equal("some-platform"))

					gr, err := gzip.NewReader(req.Body)
					Expect(err).NotTo(HaveOccurred())

					tr := tar.NewReader(gr)

					hdr, err := tr.Next()
					Expect(err).NotTo(HaveOccurred())

					Expect(hdr.Name).To(Equal("./"))

					hdr, err = tr.Next()
					Expect(err).NotTo(HaveOccurred())

					Expect(hdr.Name).To(MatchRegexp("(./)?task.yml$"))
				},
				ghttp.RespondWithJSONEncoded(201, workerArtifact),
			),
		)
		atcServer.RouteToHandler("POST", "/api/v1/teams/main/builds",
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/api/v1/teams/main/builds"),
				VerifyPlan(expectedPlan),
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
		)
		atcServer.RouteToHandler("GET", "/api/v1/builds/128/events",
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
			),
		)
		atcServer.RouteToHandler("GET", "/api/v1/builds/128/artifacts",
			ghttp.RespondWithJSONEncoded(200, []atc.WorkerArtifact{workerArtifact}),
		)

		atcServer.RouteToHandler("GET", "/api/v1/teams/main/artifacts/125", tarHandler)
	})

	Context("when running with --output", func() {
		Context("when the task specifies those outputs", func() {
			It("downloads the tasks output to the directory provided", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath, "--output", "some-dir="+outputDir)
				flyCmd.Dir = buildDir

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				// sync with after create
				Eventually(streaming).Should(BeClosed())

				close(events)

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))

				outputFiles, err := ioutil.ReadDir(outputDir)
				Expect(err).NotTo(HaveOccurred())

				Expect(outputFiles).To(HaveLen(1))
				Expect(outputFiles[0].Name()).To(Equal("some-file"))

				data, err := ioutil.ReadFile(filepath.Join(outputDir, outputFiles[0].Name()))
				Expect(err).NotTo(HaveOccurred())
				Expect(data).To(Equal([]byte("tar-contents")))
			})
		})

		Context("when the task does not specify those outputs", func() {
			It("exits 1", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "e", "-c", taskConfigPath, "-o", "wrong-output=wrong-path")
				flyCmd.Dir = buildDir

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Err).Should(gbytes.Say("error: unknown output 'wrong-output'"))

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
			})
		})
	})
})

func tarHandler(w http.ResponseWriter, req *http.Request) {
	gw := gzip.NewWriter(w)
	tw := tar.NewWriter(gw)

	tarContents := []byte("tar-contents")

	err := tw.WriteHeader(&tar.Header{
		Name: "some-file",
		Mode: 0644,
		Size: int64(len(tarContents)),
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = tw.Write(tarContents)
	Expect(err).NotTo(HaveOccurred())

	err = tw.Close()
	Expect(err).NotTo(HaveOccurred())

	err = gw.Close()
	Expect(err).NotTo(HaveOccurred())
}
