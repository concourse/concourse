package integration_test

import (
	"archive/tar"
	"compress/gzip"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/glider/api/builds"
	TurbineBuilds "github.com/concourse/turbine/api/builds"
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
	var polling chan struct{}
	var streaming chan *websocket.Conn

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
		polling = make(chan struct{})
		streaming = make(chan *websocket.Conn, 1)

		gliderServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/builds"),
				ghttp.VerifyJSONRepresenting(builds.Build{
					Path: filepath.Base(buildDir),
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
						ReadBufferSize:  1024,
						WriteBufferSize: 1024,
						CheckOrigin: func(r *http.Request) bool {
							// allow all connections
							return true
						},
					}

					conn, err := upgrader.Upgrade(w, r, nil)
					Ω(err).ShouldNot(HaveOccurred())

					streaming <- conn
				},
			),
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/builds/abc/bits"),
				func(w http.ResponseWriter, req *http.Request) {
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
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/builds/abc/result"),
				ghttp.RespondWith(200, `{"status":""}`),
				func(w http.ResponseWriter, req *http.Request) {
					close(polling)
				},
			),
		)
	})

	It("creates a build, streams output, uploads the bits, and polls until completion", func() {
		gliderServer.AllowUnhandledRequests = true

		flyCmd := exec.Command(flyPath)
		flyCmd.Dir = buildDir

		sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		var stream *websocket.Conn
		Eventually(streaming).Should(Receive(&stream))
		err = stream.WriteMessage(websocket.BinaryMessage, []byte("sup"))
		Ω(err).ShouldNot(HaveOccurred())

		Eventually(sess.Out).Should(gbytes.Say("sup"))

		Eventually(polling, 5.0).Should(BeClosed())
	})

	Context("when arguments are passed through", func() {
		BeforeEach(func() {
			gliderServer.SetHandler(
				0,
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/builds"),
					ghttp.VerifyJSONRepresenting(builds.Build{
						Path: filepath.Base(buildDir),
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

			Eventually(polling, 5.0).Should(BeClosed())
		})
	})

	Context("when the build succeeds", func() {
		BeforeEach(func() {
			gliderServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/builds/abc/result"),
					ghttp.RespondWith(200, `{"status":"succeeded"}`),
				),
			)
		})

		It("exits 0", func() {
			flyCmd := exec.Command(flyPath)
			flyCmd.Dir = buildDir

			flySession, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(flySession, 5.0).Should(gexec.Exit(0))
		})
	})

	Context("when the build fails", func() {
		BeforeEach(func() {
			gliderServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/builds/abc/result"),
					ghttp.RespondWith(200, `{"status":"failed"}`),
				),
			)
		})

		It("exits 1", func() {
			flyCmd := exec.Command(flyPath)
			flyCmd.Dir = buildDir

			flySession, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(flySession, 5.0).Should(gexec.Exit(1))
		})
	})

	Context("when the build errors", func() {
		BeforeEach(func() {
			gliderServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/builds/abc/result"),
					ghttp.RespondWith(200, `{"status":"errored"}`),
				),
			)
		})

		It("exits 2", func() {
			flyCmd := exec.Command(flyPath)
			flyCmd.Dir = buildDir

			flySession, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(flySession, 5.0).Should(gexec.Exit(2))
		})
	})
})
