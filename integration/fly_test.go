package integration_test

import (
	"archive/tar"
	"compress/gzip"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/glider/api/builds"
	TurbineBuilds "github.com/concourse/turbine/api/builds"
	"github.com/concourse/turbine/event"
)

func tarFiles(path string) string {
	output, err := exec.Command("tar", "tvf", path).Output()
	Expect(err).ToNot(HaveOccurred())

	return string(output)
}

var _ = Describe("Fly CLI", func() {
	var flyPath string
	var buildDir string

	var gliderServer *ghttp.Server
	var streaming chan *websocket.Conn
	var uploadingBits <-chan struct{}

	BeforeEach(func() {
		var err error
		flyPath, err = gexec.Build("github.com/concourse/fly")
		Ω(err).ShouldNot(HaveOccurred())

		buildDir, err = ioutil.TempDir("", "fly-build-dir")
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

		gliderServer = ghttp.NewServer()

		os.Setenv("GLIDER_URL", gliderServer.URL())
	})

	BeforeEach(func() {
		streaming = make(chan *websocket.Conn, 1)

		uploading := make(chan struct{})
		uploadingBits = uploading

		gliderServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/builds"),
				ghttp.VerifyJSONRepresenting(builds.Build{
					Name: filepath.Base(buildDir),
					Config: TurbineBuilds.Config{
						Image: "ubuntu",
						Params: map[string]string{
							"FOO": "bar",
							"BAZ": "buzz",
							"X":   "1",
						},
						Run: TurbineBuilds.RunConfig{
							Path: "find",
							Args: []string{"."},
						},
					},
				}),
				ghttp.RespondWith(201, `{
					"guid": "abc",
					"path": "some-path/",
					"config": {
						"image": "ubuntu",
						"run": {
							"path": "find",
							"args": ["."]
						},
						"params": {
							"FOO": "bar",
							"BAZ": "buzz",
							"X": "1"
						}
					}
				}`),
			),
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/builds/abc/log/output"),
				func(w http.ResponseWriter, r *http.Request) {
					upgrader := websocket.Upgrader{
						CheckOrigin: func(r *http.Request) bool {
							// allow all connections
							return true
						},
					}

					conn, err := upgrader.Upgrade(w, r, nil)
					Ω(err).ShouldNot(HaveOccurred())

					err = conn.WriteJSON(event.VersionMessage{Version: "1.0"})
					Ω(err).ShouldNot(HaveOccurred())

					streaming <- conn
				},
			),
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/builds/abc/bits"),
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
				ghttp.RespondWith(201, `{"guid":"abc","image":"ubuntu","script":"find ."}`),
			),
		)
	})

	It("creates a build, streams output, uploads the bits, and polls until completion", func() {
		flyCmd := exec.Command(flyPath)
		flyCmd.Dir = buildDir

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

	Context("when arguments are passed through", func() {
		BeforeEach(func() {
			gliderServer.SetHandler(
				0,
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/builds"),
					ghttp.VerifyJSONRepresenting(builds.Build{
						Name: filepath.Base(buildDir),
						Config: TurbineBuilds.Config{
							Image: "ubuntu",
							Params: map[string]string{
								"FOO": "bar",
								"BAZ": "buzz",
								"X":   "1",
							},
							Run: TurbineBuilds.RunConfig{
								Path: "find",
								Args: []string{".", "-name", `foo "bar" baz`},
							},
						},
					}),
					ghttp.RespondWith(201, `{
					"guid": "abc",
					"path": "some-path/",
					"config": {
						"image": "ubuntu",
						"run": {
							"path": "find",
							"args": [".", "-name", "foo \"bar\" baz"]
						},
						"params": {
							"FOO": "bar",
							"BAZ": "buzz",
							"X": "1"
						}
					}
				}`),
				),
			)
		})

		It("inserts them into the config template", func() {
			gliderServer.AllowUnhandledRequests = true

			flyCmd := exec.Command(flyPath, "--", "-name", "foo \"bar\" baz")
			flyCmd.Dir = buildDir

			_, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())

			// sync with after create
			Eventually(streaming, 5.0).Should(Receive())
		})
	})

	Context("when paramters are specified in the environment", func() {
		BeforeEach(func() {
			gliderServer.SetHandler(
				0,
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/builds"),
					ghttp.VerifyJSONRepresenting(builds.Build{
						Name: filepath.Base(buildDir),
						Config: TurbineBuilds.Config{
							Image: "ubuntu",
							Params: map[string]string{
								"FOO": "newbar",
								"BAZ": "buzz",
								"X":   "",
							},
							Run: TurbineBuilds.RunConfig{
								Path: "find",
								Args: []string{"."},
							},
						},
					}),
					ghttp.RespondWith(201, `{
					"guid": "abc",
					"path": "some-path/",
					"config": {
						"image": "ubuntu",
						"run": {
							"path": "find",
							"args": ["."]
						},
						"params": {
							"FOO": "newbar",
							"BAZ": "buzz",
							"X": ""
						}
					}
				}`),
				),
			)
		})

		It("overrides the build's paramter values", func() {
			gliderServer.AllowUnhandledRequests = true

			flyCmd := exec.Command(flyPath)
			flyCmd.Dir = buildDir
			flyCmd.Env = append(os.Environ(), "FOO=newbar", "X=")

			_, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())

			// sync with after create
			Eventually(streaming, 5.0).Should(Receive())
		})
	})

	Context("when the build is interrupted", func() {
		var aborted chan struct{}

		BeforeEach(func() {
			aborted = make(chan struct{})

			gliderServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/builds/abc/abort"),
					func(w http.ResponseWriter, r *http.Request) {
						close(aborted)
					},
				),
			)
		})

		Describe("with SIGINT", func() {
			It("aborts the build and exits nonzero", func() {
				flyCmd := exec.Command(flyPath)
				flyCmd.Dir = buildDir

				flySession, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())

				var stream *websocket.Conn
				Eventually(streaming, 5).Should(Receive(&stream))

				Eventually(uploadingBits).Should(BeClosed())

				flySession.Signal(syscall.SIGINT)

				Eventually(aborted, 5.0).Should(BeClosed())

				err = stream.WriteJSON(event.Message{
					event.Status{Status: TurbineBuilds.StatusErrored},
				})
				Ω(err).ShouldNot(HaveOccurred())

				Eventually(flySession, 5.0).Should(gexec.Exit(2))
			})
		})

		Describe("with SIGTERM", func() {
			It("aborts the build and exits nonzero", func() {
				flyCmd := exec.Command(flyPath)
				flyCmd.Dir = buildDir

				flySession, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())

				var stream *websocket.Conn
				Eventually(streaming, 5).Should(Receive(&stream))

				Eventually(uploadingBits).Should(BeClosed())

				flySession.Signal(syscall.SIGTERM)

				Eventually(aborted, 5.0).Should(BeClosed())

				err = stream.WriteJSON(event.Message{
					event.Status{Status: TurbineBuilds.StatusErrored},
				})
				Ω(err).ShouldNot(HaveOccurred())

				Eventually(flySession, 5.0).Should(gexec.Exit(2))
			})
		})
	})

	Context("when the build succeeds", func() {
		It("exits 0", func() {
			flyCmd := exec.Command(flyPath)
			flyCmd.Dir = buildDir

			flySession, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			var stream *websocket.Conn
			Eventually(streaming, 5).Should(Receive(&stream))

			err = stream.WriteJSON(event.Message{
				event.Status{Status: TurbineBuilds.StatusSucceeded},
			})
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(flySession, 5.0).Should(gexec.Exit(0))
		})
	})

	Context("when the build fails", func() {
		It("exits 1", func() {
			flyCmd := exec.Command(flyPath)
			flyCmd.Dir = buildDir

			flySession, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			var stream *websocket.Conn
			Eventually(streaming, 5).Should(Receive(&stream))

			err = stream.WriteJSON(event.Message{
				event.Status{Status: TurbineBuilds.StatusFailed},
			})
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(flySession, 5.0).Should(gexec.Exit(1))
		})
	})

	Context("when the build errors", func() {
		It("exits 2", func() {
			flyCmd := exec.Command(flyPath)
			flyCmd.Dir = buildDir

			flySession, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			var stream *websocket.Conn
			Eventually(streaming, 5).Should(Receive(&stream))

			err = stream.WriteJSON(event.Message{
				event.Status{Status: TurbineBuilds.StatusErrored},
			})
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(flySession, 5.0).Should(gexec.Exit(2))
		})
	})
})
