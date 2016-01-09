package fly_test

import (
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/concourse/testflight/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var (
	flyBin  string
	tmpHome string
)

var atcURL = "http://10.244.15.2:8080"
var targetedConcourse = "testflight"

var _ = SynchronizedBeforeSuite(func() []byte {
	flyBinPath, err := gexec.Build("github.com/concourse/fly", "-race")
	Expect(err).NotTo(HaveOccurred())

	return []byte(flyBinPath)
}, func(flyBinPath []byte) {
	flyBin = string(flyBinPath)

	var err error

	tmpHome, err = helpers.CreateTempHomeDir()
	Expect(err).NotTo(HaveOccurred())

	// observed jobs taking ~1m30s, so set the timeout pretty high
	SetDefaultEventuallyTimeout(5 * time.Minute)

	// poll less frequently
	SetDefaultEventuallyPollingInterval(time.Second)

	Eventually(errorPolling(atcURL)).ShouldNot(HaveOccurred())

	err = helpers.FlyLogin(atcURL, targetedConcourse, flyBin)
	Expect(err).NotTo(HaveOccurred())

	//For tests that require at least one build to have run
	executeSimpleTask()
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	os.RemoveAll(tmpHome)
})

func TestFly(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Authentication Fly Suite")
}

func errorPolling(url string) func() error {
	return func() error {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
		}

		return err
	}
}

func executeSimpleTask() {
	fly := exec.Command(flyBin, "-t", targetedConcourse, "execute", "-c", "../fixtures/simple-task.yml")
	session := helpers.StartFly(fly)

	Eventually(session).Should(gexec.Exit(0))
}
