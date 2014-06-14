package flight_test_test

import (
	"encoding/json"
	"os"
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

var _ = SynchronizedBeforeSuite(func() []byte {
	wardenBinPath = os.Getenv("WARDEN_BINPATH")
	Ω(wardenBinPath).ShouldNot(BeEmpty(), "must provide $WARDEN_BINPATH")

	proleBin, err := gexec.Build("github.com/winston-ci/prole", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	winstonBin, err := gexec.Build("github.com/winston-ci/winston", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	redgreenBin, err := gexec.Build("github.com/winston-ci/redgreen", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	smithBin, err := gexec.Build("github.com/winston-ci/smith", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	wardenLinuxBin, err := gexec.Build("github.com/cloudfoundry-incubator/warden-linux", "-race")
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

	processes = grouper.EnvokeGroup(grouper.RunGroup{
		"prole":        proleRunner,
		"winston":      runner.NewRunner(builtComponents["winston"]),
		"redgreen":     runner.NewRunner(builtComponents["redgreen"]),
		"warden-linux": wardenRunner,
	})

	Consistently(processes.Wait(), 5*time.Second).ShouldNot(Receive())

	os.Setenv("REDGREEN_URL", "http://127.0.0.1:5637")
})

var _ = AfterEach(func() {
	processes.Signal(syscall.SIGINT)
	Eventually(processes.Wait()).Should(Receive())
})

func TestFlightTest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FlightTest Suite")
}
