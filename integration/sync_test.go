package integration_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Syncing", func() {
	var (
		atcServer *ghttp.Server

		newFlyDir  string
		newFlyPath string
	)

	cliHandler := func() http.HandlerFunc {
		return ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", "/api/v1/cli"),
			func(w http.ResponseWriter, r *http.Request) {
				arch := r.URL.Query().Get("arch")
				platform := r.URL.Query().Get("platform")

				if arch != "amd64" && platform != runtime.GOOS {
					http.Error(w, "bad params", 500)
					return
				}

				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, "this will totally execute")
			},
		)
	}

	BeforeEach(func() {
		var err error

		newFlyDir, err = ioutil.TempDir("", "fly-sync")
		Ω(err).ShouldNot(HaveOccurred())

		newFlyPath = filepath.Join(newFlyDir, "new-fly.exe.tga.bat.legit.notavirus")

		newFly, err := os.Create(newFlyPath)
		Ω(err).ShouldNot(HaveOccurred())

		oldFly, err := os.Open(flyPath)
		Ω(err).ShouldNot(HaveOccurred())

		_, err = io.Copy(newFly, oldFly)
		Ω(err).ShouldNot(HaveOccurred())

		newFly.Close()
		oldFly.Close()

		err = os.Chmod(newFlyPath, 0755)
		Ω(err).ShouldNot(HaveOccurred())

		atcServer = ghttp.NewTLSServer()
		atcServer.AppendHandlers(cliHandler())

		os.Setenv("ATC_URL", atcServer.URL())
	})

	AfterEach(func() {
		os.RemoveAll(newFlyDir)
	})

	It("downloads and replaces the currently running executable", func() {
		flyCmd := exec.Command(newFlyPath, "-k", "sync")

		sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		<-sess.Exited
		Ω(sess.ExitCode()).Should(Equal(0))

		contents, err := ioutil.ReadFile(newFlyPath)
		Ω(err).ShouldNot(HaveOccurred())

		// don't let ginkgo try and output the entire binary as ascii
		//
		// that is the way to the dark side
		contents = contents[:8]

		expected := []byte("this will totally execute")
		Ω(contents).Should(Equal(expected[:8]))
	})
})
