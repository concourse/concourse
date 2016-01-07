package flying_test

import (
	"net/http"
	"os"

	"github.com/concourse/testflight/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
	"time"
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

	err = helpers.FlyLogin(atcURL, targetedConcourse, flyBin)
	Expect(err).NotTo(HaveOccurred())

	// observed jobs taking ~1m30s, so set the timeout pretty high
	SetDefaultEventuallyTimeout(5 * time.Minute)

	// poll less frequently
	SetDefaultEventuallyPollingInterval(time.Second)

	Eventually(errorPolling(atcURL)).ShouldNot(HaveOccurred())
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	os.RemoveAll(tmpHome)
})

func TestFlying(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Flying Suite")
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
