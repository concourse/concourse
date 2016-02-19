package integration_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var flyPath string

var homeDir string

var atcServer *ghttp.Server

const targetName = "testserver"

var _ = SynchronizedBeforeSuite(func() []byte {
	binPath, err := gexec.Build("github.com/concourse/fly")
	Expect(err).NotTo(HaveOccurred())

	return []byte(binPath)
}, func(data []byte) {
	flyPath = string(data)
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	gexec.CleanupBuildArtifacts()
})

var _ = BeforeEach(func() {
	atcServer = ghttp.NewServer()

	atcServer.AppendHandlers(
		ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", "/api/v1/auth/methods"),
			ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{}),
		),
	)

	var err error

	homeDir, err = ioutil.TempDir("", "fly-test")
	Expect(err).NotTo(HaveOccurred())

	if runtime.GOOS == "windows" {
		os.Setenv("USERPROFILE", homeDir)
	} else {
		os.Setenv("HOME", homeDir)
	}

	loginCmd := exec.Command(flyPath, "-t", targetName, "login", "-c", atcServer.URL())

	session, err := gexec.Start(loginCmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	<-session.Exited

	Expect(session.ExitCode()).To(Equal(0))
})

var _ = AfterEach(func() {
	atcServer.Close()
	os.RemoveAll(homeDir)
})

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

func tarFiles(path string) string {
	output, err := exec.Command("tar", "tvf", path).Output()
	Expect(err).ToNot(HaveOccurred())

	return string(output)
}

func osFlag(short string, long string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("/%s, /%s", short, long)
	} else {
		return fmt.Sprintf("-%s, --%s", short, long)
	}
}

func userHomeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}
