package integration_test

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/DataDog/zstd"
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
	var buildDir string
	var otherInputDir string

	var streaming chan struct{}
	var events chan atc.Event
	var uploading chan struct{}

	var expectedPlan atc.Plan
	var workerArtifact = atc.WorkerArtifact{
		ID:   125,
		Name: "some-dir",
	}

	BeforeEach(func() {
		var err error

		buildDir, err = ioutil.TempDir("", "fly-build-dir")
		Expect(err).NotTo(HaveOccurred())

		otherInputDir, err = ioutil.TempDir("", "fly-s3-asset-dir")
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(
			filepath.Join(buildDir, "task.yml"),
			[]byte(`---
platform: some-platform

image_resource:
  type: registry-image
  source:
    repository: ubuntu

inputs:
- name: some-input
- name: some-other-input

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
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(
			filepath.Join(otherInputDir, "s3-asset-file"),
			[]byte(`blob`),
			0644,
		)
		Expect(err).NotTo(HaveOccurred())

		streaming = make(chan struct{})
		events = make(chan atc.Event)

		planFactory := atc.NewPlanFactory(0)

		expectedPlan = planFactory.NewPlan(atc.DoPlan{
			planFactory.NewPlan(atc.AggregatePlan{
				planFactory.NewPlan(atc.ArtifactInputPlan{
					ArtifactID: 125,
					Name:       "some-input",
				}),
				planFactory.NewPlan(atc.GetPlan{
					Name:    "some-other-input",
					Type:    "git",
					Source:  atc.Source{"uri": "https://example.com"},
					Params:  atc.Params{"some": "other-params"},
					Version: &atc.Version{"some": "other-version"},
					Tags:    atc.Tags{"tag-1", "tag-2"},
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
						{Name: "some-input"},
						{Name: "some-other-input"},
					},
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
			}),
		})
	})

	JustBeforeEach(func() {
		uploading = make(chan struct{})
		atcServer.RouteToHandler("GET", "/api/v1/teams/main/pipelines/some-pipeline/jobs/some-job/inputs",
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/some-pipeline/jobs/some-job/inputs"),
				ghttp.RespondWithJSONEncoded(http.StatusOK, []atc.BuildInput{
					{
						Name:     "some-input",
						Type:     "git",
						Resource: "some-resource",
						Source:   atc.Source{"uri": "https://internet.com"},
						Params:   atc.Params{"some": "params"},
						Version:  atc.Version{"some": "version"},
						Tags:     atc.Tags{"tag-1", "tag-2"},
					},
					{
						Name:     "some-other-input",
						Type:     "git",
						Resource: "some-other-resource",
						Source:   atc.Source{"uri": "https://example.com"},
						Params:   atc.Params{"some": "other-params"},
						Version:  atc.Version{"some": "other-version"},
						Tags:     atc.Tags{"tag-1", "tag-2"},
					},
				}),
			),
		)
		atcServer.RouteToHandler("GET", "/api/v1/teams/main/pipelines/some-pipeline/resource-types",
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/some-pipeline/resource-types"),
				ghttp.RespondWithJSONEncoded(http.StatusOK, nil),
			),
		)
		atcServer.RouteToHandler("POST", "/api/v1/teams/main/artifacts",
			ghttp.CombineHandlers(
				func(w http.ResponseWriter, req *http.Request) {
					close(uploading)

					Expect(req.FormValue("platform")).To(Equal("some-platform"))

					tr := tar.NewReader(zstd.NewReader(req.Body))

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
		atcServer.RouteToHandler("POST", "/api/v1/teams/main/pipelines/some-pipeline/builds",
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/api/v1/teams/main/pipelines/some-pipeline/builds"),
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

	})

	It("can base inputs on a job in the pipeline", func() {
		flyCmd := exec.Command(
			flyPath, "-t", targetName, "e",
			"--inputs-from", "some-pipeline/some-job",
			"--input", fmt.Sprintf("some-input=%s", buildDir),
			"--config", filepath.Join(buildDir, "task.yml"),
		)

		sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(streaming).Should(BeClosed())
		Eventually(uploading).Should(BeClosed())

		events <- event.Log{Payload: "sup"}
		close(events)

		Eventually(sess.Out).Should(gbytes.Say("sup"))

		<-sess.Exited
		Expect(sess).To(gexec.Exit(0))
	})
})
