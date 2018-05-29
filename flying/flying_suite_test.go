package flying_test

import (
	"os"

	"github.com/concourse/go-concourse/concourse"
	"github.com/concourse/testflight/gitserver"
	"github.com/concourse/testflight/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var (
	flyBin  string
	tmpHome string

	flyHelper       *helpers.FlyHelper
	concourseClient concourse.Client
)

var atcURL = helpers.AtcURL()
var username = helpers.AtcUsername()
var password = helpers.AtcPassword()
var targetedConcourse = "testflight"

var _ = SynchronizedBeforeSuite(func() []byte {
	Eventually(helpers.ErrorPolling(atcURL)).ShouldNot(HaveOccurred())

	data, err := helpers.FirstNodeFlySetup(atcURL, targetedConcourse, username, password)
	Expect(err).NotTo(HaveOccurred())

	concourseClient := helpers.ConcourseClient(atcURL, username, password)
	gitserver.Cleanup(concourseClient)

	return data
}, func(data []byte) {
	var err error
	flyBin, tmpHome, err = helpers.AllNodeFlySetup(data)
	Expect(err).NotTo(HaveOccurred())

	flyHelper = &helpers.FlyHelper{Path: flyBin}
	concourseClient, err = helpers.AllNodeClientSetup(data)
	Expect(err).NotTo(HaveOccurred())
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	os.RemoveAll(tmpHome)
})

func TestFlying(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Flying Suite")
}
