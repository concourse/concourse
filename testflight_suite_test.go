package flight_test_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	WardenRunner "github.com/cloudfoundry-incubator/warden-linux/integration/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"

	"github.com/concourse/testflight/runner"
)

var processes ifrit.Process
var fixturesDir = "./fixtures"

var builtComponents map[string]string

var wardenBinPath string

func findSource(pkg string) string {
	for _, path := range filepath.SplitList(os.Getenv("GOPATH")) {
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
		os.Getenv("GOPATH"),
	)

	return gexec.BuildIn(gopath, pkg, args...)
}

var _ = SynchronizedBeforeSuite(func() []byte {
	wardenBinPath = os.Getenv("WARDEN_BINPATH")
	Ω(wardenBinPath).ShouldNot(BeEmpty(), "must provide $WARDEN_BINPATH")

	proleBin, err := buildWithGodeps("github.com/winston-ci/prole", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	winstonBin, err := buildWithGodeps("github.com/winston-ci/winston", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	redgreenBin, err := buildWithGodeps("github.com/winston-ci/redgreen", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	smithBin, err := buildWithGodeps("github.com/winston-ci/smith", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	wardenLinuxBin, err := buildWithGodeps("github.com/cloudfoundry-incubator/warden-linux", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	components, err := json.Marshal(map[string]string{
		"prole":        proleBin,
		"winston":      winstonBin,
		"redgreen":     redgreenBin,
		"smith":        smithBin,
		"warden-linux": wardenLinuxBin,
	})
	Ω(err).ShouldNot(HaveOccurred())

	return components
}, func(components []byte) {
	err := json.Unmarshal(components, &builtComponents)
	Ω(err).ShouldNot(HaveOccurred())
})

var _ = BeforeEach(func() {
	externalAddr := os.Getenv("EXTERNAL_ADDRESS")
	Ω(externalAddr).ShouldNot(BeEmpty(), "must specify $EXTERNAL_ADDRESS")

	wardenRunner := WardenRunner.New(
		builtComponents["warden-linux"],
		wardenBinPath,
		"bogus/rootfs",
		"-registry", "http://127.0.0.1:5000/v1/",
	)

	proleRunner := runner.NewRunner(
		builtComponents["prole"],
		"-wardenNetwork", wardenRunner.Network(),
		"-wardenAddr", wardenRunner.Addr(),
		"-resourceTypes", `{"raw":"raw-resource"}`,
	)

	redgreenRunner := runner.NewRunner(
		builtComponents["redgreen"],
		"-peerAddr", externalAddr+":5637",
	)

	processes = grouper.EnvokeGroup(grouper.RunGroup{
		"prole": proleRunner,
		//"winston":      runner.NewRunner(builtComponents["winston"]),
		"redgreen":     redgreenRunner,
		"warden-linux": wardenRunner,
	})

	Consistently(processes.Wait(), 5*time.Second).ShouldNot(Receive())

	os.Setenv("REDGREEN_URL", "http://127.0.0.1:5637")
})

var _ = AfterEach(func() {
	processes.Signal(syscall.SIGINT)
	Eventually(processes.Wait(), 10).Should(Receive())
})

func TestFlightTest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FlightTest Suite")
}
