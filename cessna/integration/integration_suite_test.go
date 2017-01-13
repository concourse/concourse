package integration_test

import (
	"fmt"
	"net/http"
	"os"

	"code.cloudfoundry.org/lager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/concourse/atc/cessna"
	"github.com/concourse/baggageclaim"

	bclient "github.com/concourse/baggageclaim/client"

	"testing"

	"archive/tar"
	"bytes"
	"io"

	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc/cessna/cessnafakes"
	"github.com/concourse/baggageclaim/baggageclaimfakes"
)

var (
	testBaseResource Resource
	worker           Worker
	baseResourceType BaseResourceType
	workerIp         string
	tarPath          string

	logger lager.Logger

	fakeWorker             *cessnafakes.FakeWorker
	fakeGardenClient       *gardenfakes.FakeClient
	fakeBaggageClaimClient *baggageclaimfakes.FakeClient
)

var _ = BeforeSuite(func() {
	_, found := os.LookupEnv("RUN_CESSNA_TESTS")
	if !found {
		Skip("Must set RUN_CESSNA_TESTS")
	}

	workerIp, found = os.LookupEnv("WORKER_IP")

	Expect(found).To(BeTrue(), "Must set WORKER_IP")

	tarPath, found = os.LookupEnv("ROOTFS_TAR_PATH")
	Expect(found).To(BeTrue(), "Must set ROOTFS_TAR_PATH")

	logger = lagertest.NewTestLogger("resource-test")

})

var _ = BeforeEach(func() {
	fakeWorker = new(cessnafakes.FakeWorker)
	fakeGardenClient = new(gardenfakes.FakeClient)
	fakeBaggageClaimClient = new(baggageclaimfakes.FakeClient)

	fakeWorker.BaggageClaimClientReturns(fakeBaggageClaimClient)
	fakeWorker.GardenClientReturns(fakeGardenClient)

	worker = NewWorker(fmt.Sprintf("%s:7777", workerIp), fmt.Sprintf("http://%s:7788", workerIp))
})

func TestResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cessna Integration Suite")
}

func createBaseResourceVolume(r io.Reader) (string, error) {
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

	err = volume.StreamIn("/", r)
	if err != nil {
		return "", err
	}

	return volume.Path(), nil
}

func NewResourceContainer(check string, in string, out string) ResourceContainer {
	return ResourceContainer{
		Check:         check,
		In:            in,
		Out:           out,
		RootFSTarPath: tarPath,
	}
}

type ResourceContainer struct {
	Check         string
	In            string
	Out           string
	RootFSTarPath string
}

func (r ResourceContainer) RootFSify() (io.Reader, error) {
	f, err := os.Open(r.RootFSTarPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buffer := new(bytes.Buffer)

	t := tar.NewWriter(buffer)
	rootFS := tar.NewReader(f)

	err = t.WriteHeader(&tar.Header{
		Name: "./opt/resource/check",
		Mode: 0755,
		Size: int64(len(r.Check)),
	})
	if err != nil {
		return nil, err
	}
	_, err = t.Write([]byte(r.Check))
	if err != nil {
		return nil, err
	}

	err = t.WriteHeader(&tar.Header{
		Name: "./opt/resource/in",
		Mode: 0755,
		Size: int64(len(r.In)),
	})
	if err != nil {
		return nil, err
	}
	_, err = t.Write([]byte(r.In))
	if err != nil {
		return nil, err
	}

	err = t.WriteHeader(&tar.Header{
		Name: "./opt/resource/out",
		Mode: 0755,
		Size: int64(len(r.Out)),
	})
	if err != nil {
		return nil, err
	}
	_, err = t.Write([]byte(r.Out))
	if err != nil {
		return nil, err
	}

	for {
		header, err := rootFS.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		err = t.WriteHeader(header)
		if err != nil {
			return nil, err
		}

		_, err = io.Copy(t, rootFS)
		if err != nil {
			return nil, err
		}
	}

	err = t.Close()
	if err != nil {
		return nil, err
	}

	return bytes.NewBuffer(buffer.Bytes()), nil
}
