package flying_test

import (
	"net/http"
	"os"

	"github.com/concourse/testflight/bosh"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
	"time"
)

var flyBin string

type GardenLinuxDeploymentData struct {
	DirectorUUID string

	GardenLinuxVersion string
}

var _ = BeforeSuite(func() {
	var err error

	gardenLinuxVersion := os.Getenv("GARDEN_LINUX_VERSION")
	Ω(gardenLinuxVersion).ShouldNot(BeEmpty(), "must set $GARDEN_LINUX_VERSION")

	flyBin, err = gexec.Build("github.com/concourse/fly", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	directorUUID := bosh.DirectorUUID()

	bosh.DeleteDeployment("concourse-testflight")

	gardenLinuxDeploymentData := GardenLinuxDeploymentData{
		DirectorUUID:       directorUUID,
		GardenLinuxVersion: gardenLinuxVersion,
	}

	bosh.Deploy("noop.yml.tmpl", gardenLinuxDeploymentData)

	Eventually(errorPolling("http://10.244.14.2:8080"), 1*time.Minute).ShouldNot(HaveOccurred())
})

func TestFlying(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Flying Suite")
}

func errorPolling(url string) func() error {
	return func() error {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
		}

		return err
	}
}
