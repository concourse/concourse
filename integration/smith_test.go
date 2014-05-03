package integration_test

import (
	"archive/tar"
	"compress/gzip"
	"io/ioutil"
	"net/http"
	"os/exec"
	"path/filepath"

	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

func tarFiles(path string) string {
	output, err := exec.Command("tar", "tvf", path).Output()
	Expect(err).ToNot(HaveOccurred())

	return string(output)
}

var _ = Describe("Smith CLI", func() {
	var smithPath string
	var buildDir string

	var redgreenServer *ghttp.Server
	var redgreenAddr string
	var polling chan struct{}
	var streaming chan *websocket.Conn

	BeforeEach(func() {
		var err error
		smithPath, err = gexec.Build("github.com/winston-ci/smith")
		Ω(err).ShouldNot(HaveOccurred())

		buildDir, err = ioutil.TempDir("", "smith-build-dir")
		Ω(err).ShouldNot(HaveOccurred())

		err = ioutil.WriteFile(
			filepath.Join(buildDir, "build.yml"),
			[]byte(`---
image: ubuntu
path: some-path/
env:
  - FOO=bar
  - BAZ=buzz
script: find . {{ .Args }}
`),
			0644,
		)
		Ω(err).ShouldNot(HaveOccurred())

		redgreenServer = ghttp.NewServer()
		redgreenAddr = redgreenServer.HTTPTestServer.Listener.Addr().String()
	})

	BeforeEach(func() {
		polling = make(chan struct{})
		streaming = make(chan *websocket.Conn, 1)

		redgreenServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/builds"),
				ghttp.VerifyJSON(`{
					"image": "ubuntu",
					"script": "find .",
					"path": "some-path/",
					"env": [
						["FOO", "bar"],
						["BAZ", "buzz"]
					]
				}`),
				ghttp.RespondWith(201, `{
					"guid": "abc",
					"image": "ubuntu",
					"script": "find .",
					"path": "some-path/",
					"env": [
						["FOO", "bar"],
						["BAZ", "buzz"]
					]
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

					Ω(hdr.Name).Should(Equal("build.yml"))
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
		redgreenServer.AllowUnhandledRequests = true

		smithCmd := exec.Command(smithPath, "-redgreenAddr", redgreenAddr)
		smithCmd.Dir = buildDir

		sess, err := gexec.Start(smithCmd, GinkgoWriter, GinkgoWriter)
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
			redgreenServer.SetHandler(
				0,
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/builds"),
					ghttp.VerifyJSON(`{
					"image": "ubuntu",
					"script": "find . \"-name\" \"foo \\\"bar\\\" baz\"",
					"path": "some-path/",
					"env": [
						["FOO", "bar"],
						["BAZ", "buzz"]
					]
				}`),
					ghttp.RespondWith(201, `{
					"guid": "abc",
					"image": "ubuntu",
					"script": "find .",
					"path": "some-path/",
					"env": [
						["FOO", "bar"],
						["BAZ", "buzz"]
					]
				}`),
				),
			)
		})

		It("inserts them into the config template", func() {
			redgreenServer.AllowUnhandledRequests = true

			smithCmd := exec.Command(smithPath, "-redgreenAddr", redgreenAddr, "--", "-name", "foo \"bar\" baz")
			smithCmd.Dir = buildDir

			_, err := gexec.Start(smithCmd, GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(polling, 5.0).Should(BeClosed())
		})
	})

	Context("when the build succeeds", func() {
		BeforeEach(func() {
			redgreenServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/builds/abc/result"),
					ghttp.RespondWith(200, `{"status":"succeeded"}`),
				),
			)
		})

		It("exits 0", func() {
			smithCmd := exec.Command(smithPath, "-redgreenAddr", redgreenAddr)
			smithCmd.Dir = buildDir

			smithSession, err := gexec.Start(smithCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(smithSession, 5.0).Should(gexec.Exit(0))
		})
	})

	Context("when the build fails", func() {
		BeforeEach(func() {
			redgreenServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/builds/abc/result"),
					ghttp.RespondWith(200, `{"status":"failed"}`),
				),
			)
		})

		It("exits 1", func() {
			smithCmd := exec.Command(smithPath, "-redgreenAddr", redgreenAddr)
			smithCmd.Dir = buildDir

			smithSession, err := gexec.Start(smithCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(smithSession, 5.0).Should(gexec.Exit(1))
		})
	})

	Context("when the build errors", func() {
		BeforeEach(func() {
			redgreenServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/builds/abc/result"),
					ghttp.RespondWith(200, `{"status":"errored"}`),
				),
			)
		})

		It("exits 2", func() {
			smithCmd := exec.Command(smithPath, "-redgreenAddr", redgreenAddr)
			smithCmd.Dir = buildDir

			smithSession, err := gexec.Start(smithCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(smithSession, 5.0).Should(gexec.Exit(2))
		})
	})
})
