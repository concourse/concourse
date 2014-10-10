package integration_test

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/atc/api/resources"
	tbuilds "github.com/concourse/turbine/api/builds"
	"github.com/concourse/turbine/event"
)

var _ = Describe("Fly CLI", func() {
	var buildDir string
	var s3AssetDir string

	var atcServer *ghttp.Server
	var streaming chan *websocket.Conn
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

		streaming = make(chan *websocket.Conn, 1)

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
					upgrader := websocket.Upgrader{
						CheckOrigin: func(r *http.Request) bool {
							// allow all connections
							return true
						},
					}

					cookie, err := r.Cookie("Some-Cookie")
					Ω(err).ShouldNot(HaveOccurred())
					Ω(cookie.Value).Should(Equal("some-cookie-data"))

					conn, err := upgrader.Upgrade(w, r, nil)
					Ω(err).ShouldNot(HaveOccurred())

					err = conn.WriteJSON(event.VersionMessage{Version: "1.0"})
					Ω(err).ShouldNot(HaveOccurred())

					streaming <- conn
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

		var stream *websocket.Conn
		Eventually(streaming).Should(Receive(&stream))

		err = stream.WriteJSON(event.Message{
			event.Log{Payload: "sup"},
		})
		Ω(err).ShouldNot(HaveOccurred())

		Eventually(sess.Out).Should(gbytes.Say("sup"))
	})
})
