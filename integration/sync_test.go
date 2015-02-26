package integration_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Syncing", func() {
	var (
		atcServer *ghttp.Server

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
		newFly, err := ioutil.TempFile("", "fly-sync")
		Ω(err).ShouldNot(HaveOccurred())
		newFly.Close()
		newFlyPath = newFly.Name()

		err = exec.Command("cp", "-a", flyPath, newFlyPath).Run()
		Ω(err).ShouldNot(HaveOccurred())

		atcServer = ghttp.NewServer()
		atcServer.AppendHandlers(cliHandler())

		os.Setenv("ATC_URL", atcServer.URL())
	})

	AfterEach(func() {
		os.RemoveAll(newFlyPath)
	})

	sync := func(args ...string) {
		flyCmd := exec.Command(newFlyPath, append([]string{"sync"}, args...)...)

		sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))
	}

	It("downloads and replaces the currently running executable", func() {
		sync()

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
