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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/vito/go-sse/sse"

	"github.com/concourse/atc/api/resources"
	tbuilds "github.com/concourse/turbine/api/builds"
	"github.com/concourse/turbine/event"
)

var _ = Describe("Fly CLI", func() {
	var buildDir string
	var s3AssetDir string

	var atcServer *ghttp.Server
	var streaming chan struct{}
	var events chan event.Event
	var uploadingBits <-chan struct{}

	var expectedTurbineBuild tbuilds.Build

	BeforeEach(func() {
		var err error

		buildDir, err = ioutil.TempDir("", "fly-build-dir")
		Ω(err).ShouldNot(HaveOccurred())

		s3AssetDir, err = ioutil.TempDir("", "fly-s3-asset-dir")
		Ω(err).ShouldNot(HaveOccurred())

		err = ioutil.WriteFile(
			filepath.Join(buildDir, "build.yml"),
			[]byte(`---
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

		err = ioutil.WriteFile(
			filepath.Join(s3AssetDir, "s3-asset-file"),
			[]byte(`blob`),
			0644,
		)
		Ω(err).ShouldNot(HaveOccurred())

		atcServer = ghttp.NewServer()

		os.Setenv("ATC_URL", atcServer.URL())

		streaming = make(chan struct{})
		events = make(chan event.Event)

		expectedTurbineBuild = tbuilds.Build{
			Config: tbuilds.Config{
				Image: "ubuntu",
				Params: map[string]string{
					"FOO": "bar",
					"BAZ": "buzz",
					"X":   "1",
				},
				Run: tbuilds.RunConfig{
					Path: "find",
					Args: []string{"."},
				},
			},

			Inputs: []tbuilds.Input{
				{
					Name: "buildDir",
					Type: "archive",
					Source: tbuilds.Source{
						"uri": "http://127.0.0.1:1234/api/v1/pipes/some-pipe-id",
					},
				},
				{
					Name: "s3Asset",
					Type: "archive",
					Source: tbuilds.Source{
						"uri": "http://127.0.0.1:1234/api/v1/pipes/some-other-pipe-id",
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
				ghttp.RespondWithJSONEncoded(http.StatusCreated, resources.Pipe{
					ID:       "some-pipe-id",
					PeerAddr: "127.0.0.1:1234",
				}),
			),
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/api/v1/pipes"),
				ghttp.RespondWithJSONEncoded(http.StatusCreated, resources.Pipe{
					ID:       "some-other-pipe-id",
					PeerAddr: "127.0.0.1:1234",
				}),
			),
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/api/v1/builds"),
				ghttp.VerifyJSONRepresenting(expectedTurbineBuild),
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
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("PUT", "/api/v1/pipes/some-other-pipe-id"),
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

					Ω(hdr.Name).Should(MatchRegexp("(./)?s3-asset-file$"))
				},
				ghttp.RespondWith(200, ""),
			),
		)
	})

	It("flies with multiple passengers", func() {
		flyCmd := exec.Command(
			flyPath,
			"--input", fmt.Sprintf("buildDir=%s", buildDir), "--input", fmt.Sprintf("s3Asset=%s", s3AssetDir),
			"--config", filepath.Join(buildDir, "build.yml"),
		)

		sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		Eventually(streaming).Should(BeClosed())

		events <- event.Log{Payload: "sup"}

		Eventually(sess.Out).Should(gbytes.Say("sup"))
	})
})
