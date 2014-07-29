package testflight_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/cloudfoundry-incubator/garden/warden"
	WardenRunner "github.com/cloudfoundry-incubator/warden-linux/integration/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"github.com/tedsuo/ifrit/grouper"
)

var externalAddr string

var processes ifrit.Process
var wardenClient warden.Client

var fixturesDir = "./fixtures"
var atcDir string

var builtComponents map[string]string

var wardenBinPath string

var helperRootfs string

var _ = SynchronizedBeforeSuite(func() []byte {
	wardenBinPath = os.Getenv("WARDEN_BINPATH")
	Ω(wardenBinPath).ShouldNot(BeEmpty(), "must provide $WARDEN_BINPATH")

	Ω(os.Getenv("BASE_GOPATH")).ShouldNot(BeEmpty(), "must provide $BASE_GOPATH")

	turbineBin, err := buildWithGodeps("github.com/concourse/turbine", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	atcBin, err := buildWithGodeps("github.com/concourse/atc", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	gliderBin, err := buildWithGodeps("github.com/concourse/glider", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	flyBin, err := buildWithGodeps("github.com/concourse/fly", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	wardenLinuxBin, err := buildWithGodeps("github.com/cloudfoundry-incubator/warden-linux", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	components, err := json.Marshal(map[string]string{
		"turbine":      turbineBin,
		"atc":          atcBin,
		"glider":       gliderBin,
		"fly":          flyBin,
		"warden-linux": wardenLinuxBin,
	})
	Ω(err).ShouldNot(HaveOccurred())

	return components
}, func(components []byte) {
	err := json.Unmarshal(components, &builtComponents)
	Ω(err).ShouldNot(HaveOccurred())

	atcDir = findSource("github.com/concourse/atc")
})

var _ = BeforeEach(func() {
	externalAddr = os.Getenv("EXTERNAL_ADDRESS")
	Ω(externalAddr).ShouldNot(BeEmpty(), "must specify $EXTERNAL_ADDRESS")

	rawResourceRootfs := os.Getenv("RAW_RESOURCE_ROOTFS")
	Ω(rawResourceRootfs).ShouldNot(BeEmpty(), "must specify $RAW_RESOURCE_ROOTFS")

	gitResourceRootfs := os.Getenv("GIT_RESOURCE_ROOTFS")
	Ω(gitResourceRootfs).ShouldNot(BeEmpty(), "must specify $GIT_RESOURCE_ROOTFS")

	helperRootfs = os.Getenv("HELPER_ROOTFS")
	Ω(helperRootfs).ShouldNot(BeEmpty(), "must specify $HELPER_ROOTFS")

	wardenAddr := fmt.Sprintf("127.0.0.1:%d", 4859+GinkgoParallelNode())

	wardenRunner := WardenRunner.New(
		"tcp",
		wardenAddr,
		builtComponents["warden-linux"],
		wardenBinPath,
		"bogus/rootfs",
	)

	wardenClient = wardenRunner.NewClient()

	turbineRunner := &ginkgomon.Runner{
		Name:          "turbine",
		AnsiColorCode: "33m",
		Command: exec.Command(
			builtComponents["turbine"],
			"-wardenNetwork", "tcp",
			"-wardenAddr", wardenAddr,
			"-resourceTypes", fmt.Sprintf(`{
				"raw": "%s",
				"git": "%s"
			}`, rawResourceRootfs, gitResourceRootfs),
		),
		StartCheck:        "listening",
		StartCheckTimeout: 30 * time.Second,
	}

	gliderRunner := &ginkgomon.Runner{
		Name:          "glider",
		AnsiColorCode: "32m",
		Command: exec.Command(
			builtComponents["glider"],
			"-peerAddr", externalAddr+":5637",
		),
		StartCheck: "listening",
	}

	processes = grouper.EnvokeGroup(grouper.RunGroup{
		"turbine":      turbineRunner,
		"glider":       gliderRunner,
		"warden-linux": wardenRunner,
	})

	Consistently(processes.Wait(), 1*time.Second).ShouldNot(Receive())

	os.Setenv("GLIDER_URL", "http://127.0.0.1:5637")
})

var _ = AfterEach(func() {
	processes.Signal(syscall.SIGINT)
	Eventually(processes.Wait(), 10*time.Second).Should(Receive())
})

func TestFlightTest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FlightTest Suite")
}

func findSource(pkg string) string {
	for _, path := range filepath.SplitList(os.Getenv("BASE_GOPATH")) {
		srcPath := filepath.Join(path, "src", pkg)

		_, err := os.Stat(srcPath)
		if err != nil {
			continue
		}

		return srcPath
	}

	return ""
}

func buildWithGodeps(pkg string, args ...string) (string, error) {
	srcPath := findSource(pkg)
	Ω(srcPath).ShouldNot(BeEmpty(), "could not find source for "+pkg)

	gopath := fmt.Sprintf(
		"%s%c%s",
		filepath.Join(srcPath, "Godeps", "_workspace"),
		os.PathListSeparator,
		os.Getenv("BASE_GOPATH"),
	)

	return gexec.BuildIn(gopath, pkg, args...)
}
