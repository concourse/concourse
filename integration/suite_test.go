package integration_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
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
const atcVersion = "1.2.3"

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

func infoHandler() http.HandlerFunc {
	return ghttp.CombineHandlers(
		ghttp.VerifyRequest("GET", "/api/v1/info"),
		ghttp.RespondWithJSONEncoded(200, atc.Info{Version: atcVersion}),
	)
}

func tokenHandler(teamName string) http.HandlerFunc {
	return ghttp.CombineHandlers(
		ghttp.VerifyRequest("GET", "/api/v1/teams/"+teamName+"/auth/token"),
		ghttp.RespondWithJSONEncoded(
			200,
			token(),
		),
	)
}

func tokenString() string {
	return string(token().Type) + " " + string(token().Value)
}

func token() atc.AuthToken {
	return atc.AuthToken{
		Type:  "Bearer",
		Value: "some-token",
	}
}

var _ = BeforeEach(func() {
	atcServer = ghttp.NewServer()

	atcServer.AppendHandlers(
		infoHandler(),
		ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", "/api/v1/teams/main/auth/methods"),
			ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{}),
		),
		tokenHandler("main"),
		infoHandler(),
	)

	var err error

	homeDir, err = ioutil.TempDir("", "fly-test")
	Expect(err).NotTo(HaveOccurred())

	if runtime.GOOS == "windows" {
		os.Setenv("USERPROFILE", homeDir)
	} else {
		os.Setenv("HOME", homeDir)
	}

	loginCmd := exec.Command(flyPath, "-t", targetName, "login", "-c", atcServer.URL(), "-n", "main")

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

func Change(fn func() int) *changeMatcher {
	return &changeMatcher{
		fn: fn,
	}
}

type changeMatcher struct {
	fn     func() int
	amount int

	before int
	after  int
}

func (cm *changeMatcher) By(amount int) *changeMatcher {
	cm.amount = amount

	return cm
}

func (cm *changeMatcher) Match(actual interface{}) (success bool, err error) {
	cm.before = cm.fn()

	ac, ok := actual.(func())
	if !ok {
		return false, errors.New("expected a function")
	}

	ac()

	cm.after = cm.fn()

	return (cm.after - cm.before) == cm.amount, nil
}

func (cm *changeMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected value to change by %d but it changed from %d to %d", cm.amount, cm.before, cm.after)
}

func (cm *changeMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected value not to change by %d but it changed from %d to %d", cm.amount, cm.before, cm.after)
}
