package flying_test

import (
	"os"

	"github.com/concourse/testflight/bosh"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var flyBin string

var _ = BeforeSuite(func() {
	Ω(os.Getenv("BOSH_LITE_IP")).ShouldNot(BeEmpty(), "must specify $BOSH_LITE_IP")

	deployment := bosh.DeployConcourse("noop.yml")

	var err error

	flyBin, err = gexec.Build("github.com/concourse/fly", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	os.Setenv("ATC_URL", deployment.ATCUrl)
})

func TestFlying(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Flying Suite")
}
