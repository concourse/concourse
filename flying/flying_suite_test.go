package flying_test

import (
	"net/http"
	"os"

	gapi "github.com/cloudfoundry-incubator/garden/api"
	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/concourse/testflight/bosh"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
	"time"
)

var flyBin string

var _ = BeforeSuite(func() {
	Ω(os.Getenv("BOSH_LITE_IP")).ShouldNot(BeEmpty(), "must specify $BOSH_LITE_IP")

	var err error

	flyBin, err = gexec.Build("github.com/concourse/fly", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	bosh.DeleteDeployment("concourse")

	bosh.Deploy("noop.yml")

	atcURL := "http://" + os.Getenv("BOSH_LITE_IP") + ":8080"

	os.Setenv("ATC_URL", atcURL)

	Eventually(func() error {
		resp, err := http.Get(atcURL)
		if err == nil {
			resp.Body.Close()
		}

		return err
	}, 1*time.Minute).ShouldNot(HaveOccurred())

	gardenClient := client.New(connection.New("tcp", os.Getenv("BOSH_LITE_IP")+":7777"))
	Eventually(gardenClient.Ping, 10*time.Second).ShouldNot(HaveOccurred())

	// warm cache with testflight-helper image so flying doesn't take forever
	container, err := gardenClient.Create(gapi.ContainerSpec{
		RootFSPath: "docker:///concourse/testflight-helper",
	})
	Ω(err).ShouldNot(HaveOccurred())

	err = gardenClient.Destroy(container.Handle())
	Ω(err).ShouldNot(HaveOccurred())
})

func TestFlying(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Flying Suite")
}
