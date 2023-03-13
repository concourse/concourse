package integration_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/concourse/concourse/fly/version"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Syncing", func() {
	var (
		flyVersion    string
		copiedFlyDir  string
		copiedFlyPath string
	)

	BeforeEach(func() {
		copiedFlyDir, err := ioutil.TempDir("", "fly_sync")
		Expect(err).ToNot(HaveOccurred())

		copiedFly, err := os.Create(filepath.Join(copiedFlyDir, filepath.Base(flyPath)))
		Expect(err).ToNot(HaveOccurred())

		fly, err := os.Open(flyPath)
		Expect(err).ToNot(HaveOccurred())

		_, err = io.Copy(copiedFly, fly)
		Expect(err).ToNot(HaveOccurred())

		Expect(copiedFly.Close()).To(Succeed())

		Expect(fly.Close()).To(Succeed())

		copiedFlyPath = copiedFly.Name()

		fi, err := os.Stat(flyPath)
		Expect(err).ToNot(HaveOccurred())

		Expect(os.Chmod(copiedFlyPath, fi.Mode())).To(Succeed())

		atcServer.AppendHandlers(ghttp.CombineHandlers(
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
		))
	})

	AfterEach(func() {
		Expect(os.RemoveAll(copiedFlyDir)).To(Succeed())
	})

	downloadAndReplaceExecutable := func(arg ...string) {
		flyCmd := exec.Command(copiedFlyPath, arg...)
		flyCmd.Env = append(os.Environ(), "FAKE_FLY_VERSION="+flyVersion)

		sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		<-sess.Exited
		Expect(sess.ExitCode()).To(Equal(0))

		expected := []byte("this will totally execute")
		expectBinaryToMatch(copiedFlyPath, expected[:8])
	}

	Context("When versions mismatch between fly + atc", func() {
		BeforeEach(func() {
			major, minor, patch, err := version.GetSemver(atcVersion)
			Expect(err).NotTo(HaveOccurred())

			flyVersion = fmt.Sprintf("%d.%d.%d", major, minor, patch+1)
		})

		It("downloads and replaces the currently running executable with target", func() {
			downloadAndReplaceExecutable("-t", targetName, "sync")
		})

		It("downloads and replaces the currently running executable with target URL", func() {
			downloadAndReplaceExecutable("sync", "-c", atcServer.URL())
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

				Expect(os.Chmod(filepath.Dir(copiedFlyPath), 0500)).To(Succeed())

				expectedBinary := readBinary(copiedFlyPath)

				flyCmd := exec.Command(copiedFlyPath, "sync", "-c", atcServer.URL())
				flyCmd.Env = append(os.Environ(), "FAKE_FLY_VERSION="+flyVersion)

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
				Expect(sess.Err).To(gbytes.Say("update failed.*permission denied"))

				expectBinaryToMatch(copiedFlyPath, expectedBinary)
			})
		})
	})

	Context("When versions match between fly + atc", func() {
		BeforeEach(func() {
			flyVersion = atcVersion
		})

		It("informs the user, and doesn't download/replace the executable", func() {
			expectedBinary := readBinary(copiedFlyPath)

			flyCmd := exec.Command(copiedFlyPath, "sync", "-c", atcServer.URL())
			flyCmd.Env = append(os.Environ(), "FAKE_FLY_VERSION="+flyVersion)

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))
			Expect(sess.Out).To(gbytes.Say(`version 6.3.1 already matches; skipping`))

			expectBinaryToMatch(copiedFlyPath, expectedBinary)
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
