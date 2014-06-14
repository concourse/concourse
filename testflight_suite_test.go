package flight_test_test

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
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
	"github.com/concourse/testflight/staticregistry"
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
	ubuntuTarball := os.Getenv("UBUNTU_IMAGE_TARBALL")
	Ω(ubuntuTarball).ShouldNot(BeEmpty(), "must specify $UBUNTU_IMAGE_TARBALL")

	rawResourceTarball := os.Getenv("RAW_RESOURCE_IMAGE_TARBALL")
	Ω(rawResourceTarball).ShouldNot(BeEmpty(), "must specify $RAW_RESOURCE_IMAGE_TARBALL")

	staticRegistry := staticregistry.Registry{
		ImageTarball:       ubuntuTarball,
		RawResourceTarball: rawResourceTarball,
	}

	mux := http.NewServeMux()

	mux.HandleFunc(
		"/repositories/ubuntu/images",
		staticRegistry.UbuntuImages,
	)

	mux.HandleFunc(
		"/v1/repositories/library/ubuntu/tags",
		staticRegistry.UbuntuTags,
	)

	mux.HandleFunc(
		"/v1/images/ubuntu-id/ancestry",
		staticRegistry.UbuntuAncestry,
	)

	mux.HandleFunc(
		"/v1/images/ubuntu-layer/json",
		staticRegistry.UbuntuLayerJSON,
	)

	mux.HandleFunc(
		"/v1/images/ubuntu-layer/layer",
		staticRegistry.UbuntuLayerTarball,
	)

	mux.HandleFunc(
		"/repositories/winston/raw-resource/images",
		staticRegistry.RawResourceImages,
	)

	mux.HandleFunc(
		"/v1/repositories/winston/raw-resource/tags",
		staticRegistry.RawResourceTags,
	)

	mux.HandleFunc(
		"/v1/images/raw-resource-id/ancestry",
		staticRegistry.RawResourceAncestry,
	)

	mux.HandleFunc(
		"/v1/images/raw-resource-layer/json",
		staticRegistry.RawResourceLayerJSON,
	)

	mux.HandleFunc(
		"/v1/images/raw-resource-layer/layer",
		staticRegistry.RawResourceLayerTarball,
	)

	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("GOT REQUEST", r.URL.String())
		mux.ServeHTTP(w, r)
	}))

	wardenRunner := WardenRunner.New(
		builtComponents["warden-linux"],
		wardenBinPath,
		"bogus/rootfs",
		"-registry", registry.URL+"/", // :(
	)

	proleRunner := runner.NewRunner(
		builtComponents["prole"],
		"-wardenNetwork", wardenRunner.Network(),
		"-wardenAddr", wardenRunner.Addr(),
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
