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

	var err error

	flyBin, err = gexec.Build("github.com/concourse/fly", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	bosh.DeleteDeployment("concourse")

	bosh.Deploy("noop.yml")

	os.Setenv("ATC_URL", "http://"+os.Getenv("BOSH_LITE_IP")+":8080")
})

func TestFlying(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Flying Suite")
}
