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
		Expect(err).NotTo(HaveOccurred())

		newFlyPath = filepath.Join(newFlyDir, "new-fly.exe.tga.bat.legit.notavirus")

		newFly, err := os.Create(newFlyPath)
		Expect(err).NotTo(HaveOccurred())

		oldFly, err := os.Open(flyPath)
		Expect(err).NotTo(HaveOccurred())

		_, err = io.Copy(newFly, oldFly)
		Expect(err).NotTo(HaveOccurred())

		newFly.Close()
		oldFly.Close()

		err = os.Chmod(newFlyPath, 0755)
		Expect(err).NotTo(HaveOccurred())

		// replace info handler with sync handler, since sync does not verify client version
		atcServer.SetHandler(3, cliHandler())
	})

	AfterEach(func() {
		os.RemoveAll(newFlyDir)
	})

	It("downloads and replaces the currently running executable", func() {
		flyCmd := exec.Command(newFlyPath, "-t", targetName, "sync")

		sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		<-sess.Exited
		Expect(sess.ExitCode()).To(Equal(0))

		contents, err := ioutil.ReadFile(newFlyPath)
		Expect(err).NotTo(HaveOccurred())

		// don't let ginkgo try and output the entire binary as ascii
		//
		// that is the way to the dark side
		contents = contents[:8]

		expected := []byte("this will totally execute")
		Expect(contents).To(Equal(expected[:8]))
	})
})
