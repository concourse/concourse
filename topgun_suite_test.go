package topgun_test

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"code.cloudfoundry.org/lager/lagertest"

	"github.com/concourse/go-concourse/concourse"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var (
	boshEnv string

	deploymentName, flyTarget string

	atcIP, atcExternalURL string

	pipelineName string

	tmpHome string
	flyBin  string

	client concourse.Client
	team   concourse.Team

	logger *lagertest.TestLogger
)

func TestTOPGUN(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TOPGUN Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	flyBinPath, err := gexec.Build("github.com/concourse/fly")
	Expect(err).ToNot(HaveOccurred())

	return []byte(flyBinPath)
}, func(data []byte) {
	flyBin = string(data)
})

var _ = SynchronizedAfterSuite(func() {
	gexec.CleanupBuildArtifacts()
}, func() {})

var _ = BeforeEach(func() {
	SetDefaultEventuallyTimeout(5 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)
	SetDefaultConsistentlyDuration(time.Minute)
	SetDefaultConsistentlyPollingInterval(time.Second)

	logger = lagertest.NewTestLogger("test")

	boshEnv = os.Getenv("BOSH_ENV")
	if boshEnv == "" {
		Fail("must specify $BOSH_ENV")
	}

	deploymentName = fmt.Sprintf("concourse-topgun-%d", GinkgoParallelNode())
	flyTarget = deploymentName

	bosh("delete-deployment")

	atcIP = fmt.Sprintf("10.234.%d.2", GinkgoParallelNode())
	atcExternalURL = fmt.Sprintf("http://%s:8080", atcIP)

	client = concourse.NewClient(atcExternalURL, nil)
	team = client.Team("main")
})

var _ = AfterEach(func() {
	bosh("delete-deployment")
})

func Deploy(manifest string) {
	bosh(
		"deploy", manifest,
		"-v", "deployment-name="+deploymentName,
		"-v", "atc-ip="+atcIP,
		"-v", "atc-external-url="+atcExternalURL,
	)

	fly("login", "-c", atcExternalURL)
}

func bosh(argv ...string) {
	run("bosh", append([]string{"-n", "-e", boshEnv, "-d", deploymentName}, argv...)...)
}

func fly(argv ...string) {
	run("fly", append([]string{"-t", flyTarget}, argv...)...)
}

func run(argc string, argv ...string) {
	cmd := exec.Command(argc, argv...)
	cmd.Stdout = GinkgoWriter
	cmd.Stderr = GinkgoWriter

	err := cmd.Run()
	Expect(err).ToNot(HaveOccurred())
}
