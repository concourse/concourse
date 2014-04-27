package integration_test

import (
	"archive/tar"
	"compress/gzip"
	"io/ioutil"
	"net/http"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
	var polling chan struct{}

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
script: find .
`),
			0644,
		)
		Ω(err).ShouldNot(HaveOccurred())

		redgreenServer = ghttp.NewServer()
	})

	BeforeEach(func() {
		polling = make(chan struct{})

		redgreenServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/builds"),
				ghttp.VerifyJSON(`{"image": "ubuntu", "script": "find .", "path": "."}`),
				ghttp.RespondWith(201, `{"guid":"abc","image":"ubuntu","script":"find .","path":"."}`),
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

	It("creates a build, uploads the bits, and polls until completion", func() {
		redgreenServer.AllowUnhandledRequests = true

		smithCmd := exec.Command(smithPath, "-redgreenURL", redgreenServer.URL())
		smithCmd.Dir = buildDir

		_, err := gexec.Start(smithCmd, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		Eventually(polling, 5.0).Should(BeClosed())
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
			smithCmd := exec.Command(smithPath, "-redgreenURL", redgreenServer.URL())
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
			smithCmd := exec.Command(smithPath, "-redgreenURL", redgreenServer.URL())
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
			smithCmd := exec.Command(smithPath, "-redgreenURL", redgreenServer.URL())
			smithCmd.Dir = buildDir

			smithSession, err := gexec.Start(smithCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(smithSession, 5.0).Should(gexec.Exit(2))
		})
	})
})
