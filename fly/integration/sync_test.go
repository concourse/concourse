package integration_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/concourse/fly/version"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Syncing", func() {
	var (
		flyVersion string
		flyPath    string
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

	JustBeforeEach(func() {
		var err error
		flyPath, err = gexec.Build(
			"github.com/concourse/fly",
			"-ldflags", fmt.Sprintf("-X github.com/concourse/fly/version.Version=%s", flyVersion),
		)
		Expect(err).NotTo(HaveOccurred())

		atcServer.AppendHandlers(cliHandler())
	})

	Context("When versions mismatch between fly + atc", func() {
		BeforeEach(func() {
			major, minor, patch, err := version.GetSemver(atcVersion)
			Expect(err).NotTo(HaveOccurred())

			flyVersion = fmt.Sprintf("%d.%d.%d", major, minor, patch+1)
		})
		It("downloads and replaces the currently running executable", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "sync")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))

			expected := []byte("this will totally execute")
			expectBinaryToMatch(flyPath, expected[:8])
		})

		Context("When the user running sync doesn't have write permissions for the target directory", func() {
			It("returns an error, and doesn't download/replace the executable", func() {
				me, err := user.Current()
				Expect(err).ToNot(HaveOccurred())

				if me.Uid == "0" {
					Skip("root can always write; not worth testing")
					return
				}

				if runtime.GOOS == "windows" {
					Skip("who knows how windows works; not worth testing")
					return
				}

				os.Chmod(filepath.Dir(flyPath), 0500)

				expectedBinary := readBinary(flyPath)

				flyCmd := exec.Command(flyPath, "-t", targetName, "sync")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
				Expect(sess.Err).To(gbytes.Say("update failed.*permission denied"))

				expectBinaryToMatch(flyPath, expectedBinary)
			})
		})
	})

	Context("When versions match between fly + atc", func() {
		BeforeEach(func() {
			flyVersion = atcVersion
		})
		It("informs the user, and doesn't download/replace the executable", func() {
			expectedBinary := readBinary(flyPath)

			flyCmd := exec.Command(flyPath, "-t", targetName, "sync")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))
			Expect(sess.Out).To(gbytes.Say(`version 4.0.0 already matches; skipping`))

			expectBinaryToMatch(flyPath, expectedBinary)
		})
	})
})

func readBinary(path string) []byte {
	expectedBinary, err := ioutil.ReadFile(flyPath)
	Expect(err).NotTo(HaveOccurred())
	return expectedBinary[:8]
}

func expectBinaryToMatch(path string, expectedBinary []byte) {
	contents, err := ioutil.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())

	// don't let ginkgo try and output the entire binary as ascii
	//
	// that is the way to the dark side
	contents = contents[:8]
	Expect(contents).To(Equal(expectedBinary))
}
