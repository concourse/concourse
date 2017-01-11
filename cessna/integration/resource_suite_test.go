package integration_test

import (
	"fmt"
	"net/http"
	"os"

	"code.cloudfoundry.org/lager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/cessna"
	. "github.com/concourse/atc/cessna/resource"
	"github.com/concourse/baggageclaim"

	bclient "github.com/concourse/baggageclaim/client"

	"testing"

	"code.cloudfoundry.org/lager/lagertest"
)

var (
	testBaseResource Resource
	testWorker       *cessna.Worker
	baseResourceType BaseResourceType
	workerIp         string
	tarPath          string

	logger lager.Logger
)

var _ = BeforeSuite(func() {
	_, found := os.LookupEnv("RUN_CESSNA_TESTS")
	if !found {
		Skip("Must set RUN_CESSNA_TESTS")
	}

	workerIp, found = os.LookupEnv("WORKER_IP")

	Expect(found).To(BeTrue(), "Must set WORKER_IP")

	tarPath, found = os.LookupEnv("TAR_PATH")
	Expect(found).To(BeTrue(), "Must set TAR_PATH")

	testWorker = cessna.NewWorker(fmt.Sprintf("%s:7777", workerIp), fmt.Sprintf("http://%s:7788", workerIp))

	rootFSPath, err := createBaseResourceVolume(tarPath)

	Expect(err).ToNot(HaveOccurred())

	baseResourceType = BaseResourceType{
		RootFSPath: rootFSPath,
		Name:       "echo",
	}

	logger = lagertest.NewTestLogger("resource-test")
})

func TestResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resource Suite")

}

func createBaseResourceVolume(tarPath string) (string, error) {
	baggageclaimClient := bclient.New(fmt.Sprintf("http://%s:7788", workerIp), http.DefaultTransport)

	volumeSpec := baggageclaim.VolumeSpec{
		Strategy:   baggageclaim.EmptyStrategy{},
		Privileged: true,
	}

	volume, err := baggageclaimClient.CreateVolume(
		lager.NewLogger("create-volume-for-base-resource"),
		volumeSpec)

	if err != nil {
		return "", err
	}

	tarfile, err := os.Open(tarPath)
	if err != nil {
		return "", err
	}

	err = volume.StreamIn("/", tarfile)
	if err != nil {
		return "", err
	}

	return volume.Path(), nil
}
