package fly_test

import (
	"os"
	"os/exec"

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

var atcURL = helpers.AtcURL()
var targetedConcourse = "testflight"

var _ = SynchronizedBeforeSuite(func() []byte {
	data, err := helpers.FirstNodeFlySetup(atcURL, targetedConcourse)
	Expect(err).NotTo(HaveOccurred())

	return data
}, func(data []byte) {
	Eventually(helpers.ErrorPolling(atcURL)).ShouldNot(HaveOccurred())

	var err error
	flyBin, tmpHome, err = helpers.AllNodeFlySetup(data)
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

func executeSimpleTask() {
	fly := exec.Command(flyBin, "-t", targetedConcourse, "execute", "-c", "../fixtures/simple-task.yml")
	session := helpers.StartFly(fly)

	Eventually(session).Should(gexec.Exit(0))
}
