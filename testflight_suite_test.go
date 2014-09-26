package testflight_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"text/template"
	"time"

	"github.com/cloudfoundry-incubator/garden/warden"
	"github.com/concourse/atc/postgresrunner"
	"github.com/concourse/testflight/gardenrunner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"github.com/tedsuo/ifrit/grouper"
)

var (
	externalAddress string
	gardenBinPath   string
	helperRootfs    string

	builtComponents map[string]string

	atcDir              string
	atcPipelineFilePath string
	atcRunner           ifrit.Runner

	postgresRunner postgresrunner.Runner

	plumbing     ifrit.Process
	gardenClient warden.Client

	atcProcess ifrit.Process
)

var _ = SynchronizedBeforeSuite(func() []byte {
	gardenBinPath = os.Getenv("GARDEN_BINPATH")
	Ω(gardenBinPath).ShouldNot(BeEmpty(), "must provide $GARDEN_BINPATH")

	turbineBin, err := gexec.Build("github.com/concourse/turbine", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	atcBin, err := gexec.Build("github.com/concourse/atc", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	flyBin, err := gexec.Build("github.com/concourse/fly", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	gardenLinuxBin, err := buildWithGodeps("github.com/cloudfoundry-incubator/garden-linux", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	components, err := json.Marshal(map[string]string{
		"turbine":      turbineBin,
		"atc":          atcBin,
		"fly":          flyBin,
		"garden-linux": gardenLinuxBin,
	})
	Ω(err).ShouldNot(HaveOccurred())

	return components
}, func(components []byte) {
	err := json.Unmarshal(components, &builtComponents)
	Ω(err).ShouldNot(HaveOccurred())
})

var _ = BeforeEach(func() {
	atcDir = os.Getenv("ATC_DIR")
	Ω(atcDir).ShouldNot(BeEmpty(), "must specify $ATC_DIR")

	externalAddress = os.Getenv("EXTERNAL_ADDRESS")
	Ω(externalAddress).ShouldNot(BeEmpty(), "must specify $EXTERNAL_ADDRESS")

	archiveResourceRootfs := os.Getenv("ARCHIVE_RESOURCE_ROOTFS")
	Ω(archiveResourceRootfs).ShouldNot(BeEmpty(), "must specify $ARCHIVE_RESOURCE_ROOTFS")

	gitResourceRootfs := os.Getenv("GIT_RESOURCE_ROOTFS")
	Ω(gitResourceRootfs).ShouldNot(BeEmpty(), "must specify $GIT_RESOURCE_ROOTFS")

	helperRootfs = os.Getenv("HELPER_ROOTFS")
	Ω(helperRootfs).ShouldNot(BeEmpty(), "must specify $HELPER_ROOTFS")

	gardenAddr := fmt.Sprintf("127.0.0.1:%d", 4859+GinkgoParallelNode())

	gardenRunner := gardenrunner.New(
		"tcp",
		gardenAddr,
		builtComponents["garden-linux"],
		gardenBinPath,
		"bogus/rootfs",
		"/tmp",
	)

	gardenClient = gardenRunner.NewClient()

	turbineRunner := &ginkgomon.Runner{
		Name:          "turbine",
		AnsiColorCode: "33m",
		Command: exec.Command(
			builtComponents["turbine"],
			"-gardenNetwork", "tcp",
			"-gardenAddr", gardenAddr,
			"-resourceTypes", fmt.Sprintf(`{
				"archive": "%s",
				"git": "%s"
			}`, archiveResourceRootfs, gitResourceRootfs),
		),
		StartCheck:        "listening",
		StartCheckTimeout: 30 * time.Second,
	}

	postgresRunner = postgresrunner.Runner{
		Port: 5433 + GinkgoParallelNode(),
	}

	atcPipelineFilePath = fmt.Sprintf("/tmp/atc-pipeline-%d", GinkgoParallelNode())

	atcRunner = &ginkgomon.Runner{
		Name:          "atc",
		AnsiColorCode: "34m",
		Command: exec.Command(
			builtComponents["atc"],
			"-callbacksURL", "http://"+externalAddress,
			"-pipeline", atcPipelineFilePath,
			"-templates", filepath.Join(atcDir, "web", "templates"),
			"-public", filepath.Join(atcDir, "web", "public"),
			"-sqlDataSource", postgresRunner.DataSourceName(),
			"-checkInterval", "5s",
			"-dev",
		),
		StartCheck:        "listening",
		StartCheckTimeout: 5 * time.Second,
	}

	os.Setenv("ATC_URL", "http://127.0.0.1:8080")

	plumbing = ifrit.Invoke(grouper.NewParallel(os.Interrupt, []grouper.Member{
		{"turbine", turbineRunner},
		{"garden-linux", gardenRunner},
		{"postgres", postgresRunner},
	}))

	Consistently(plumbing.Wait(), 1*time.Second).ShouldNot(Receive())

	postgresRunner.CreateTestDB()
})

var _ = AfterEach(func() {
	stopProcess(atcProcess)

	postgresRunner.DropTestDB()

	stopProcess(plumbing)

	err := os.Remove(atcPipelineFilePath)
	Ω(err).ShouldNot(HaveOccurred())
})

func TestTestFlight(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TestFlight Suite")
}

func stopProcess(process ifrit.Process) {
	process.Signal(syscall.SIGINT)

	select {
	case <-process.Wait():
	case <-time.After(10 * time.Second):
		println("!!!!!!!!!!!!!!!!!!!!!!!!!!!! EXIT TIMEOUT")

		process.Signal(syscall.SIGQUIT)
		Eventually(process.Wait(), 10*time.Second).Should(Receive())

		Fail("processes did not exit within 10s; SIGQUIT sent")
	}
}

func writeATCPipeline(templateName string, templateData interface{}) {
	gitPipelineTemplate, err := template.ParseFiles("pipelines/" + templateName)
	Ω(err).ShouldNot(HaveOccurred())

	atcPipelineFile, err := os.Create(atcPipelineFilePath)
	Ω(err).ShouldNot(HaveOccurred())

	err = gitPipelineTemplate.Execute(atcPipelineFile, templateData)
	Ω(err).ShouldNot(HaveOccurred())

	err = atcPipelineFile.Close()
	Ω(err).ShouldNot(HaveOccurred())
}

func buildWithGodeps(pkg string, args ...string) (string, error) {
	gopath := fmt.Sprintf(
		"%s%c%s",
		filepath.Join(os.Getenv("BASE_GOPATH"), "src", pkg, "Godeps", "_workspace"),
		os.PathListSeparator,
		os.Getenv("BASE_GOPATH"),
	)

	return gexec.BuildIn(gopath, pkg, args...)
}
