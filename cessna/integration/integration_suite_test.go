package integration_test

import (
	"fmt"
	"net/http"
	"os"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"

	uuid "github.com/nu7hatch/gouuid"
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
	gserver "code.cloudfoundry.org/garden/server"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc/cessna/cessnafakes"
	"github.com/concourse/baggageclaim/baggageclaimfakes"
)

var (
	skipped bool

	worker   Worker
	workerIp string
	tarPath  string
	tarURL   string

	found bool

	logger lager.Logger

	fakeWorker             *cessnafakes.FakeWorker
	fakeGardenClient       *gardenfakes.FakeClient
	fakeBaggageClaimClient *baggageclaimfakes.FakeClient
)

var _ = BeforeSuite(func() {
	_, found = os.LookupEnv("RUN_CESSNA_TESTS")
	if !found {
		skipped = true
	}
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	if skipped || workerIp == "" {
		return
	}

	worker = NewWorker(fmt.Sprintf("%s:7777", workerIp), fmt.Sprintf("http://%s:7788", workerIp))

	containers, err := worker.GardenClient().Containers(nil)
	Expect(err).NotTo(HaveOccurred())

	for _, container := range containers {
		err = worker.GardenClient().Destroy(container.Handle())
		if err != nil {
			// once garden fixes container grace timeout to be indefinite we can remove this check
			if _, ok := err.(garden.ContainerNotFoundError); !ok {
				if err.Error() != gserver.ErrConcurrentDestroy.Error() {
					Expect(err).NotTo(HaveOccurred())
				}
			}
		}
	}

	volumes, err := worker.BaggageClaimClient().ListVolumes(logger, nil)
	Expect(err).NotTo(HaveOccurred())

	for _, volume := range volumes {
		err = volume.Destroy()
		if err != nil && err != baggageclaim.ErrVolumeNotFound {
			Expect(err).NotTo(HaveOccurred())
		}
	}
})

var _ = BeforeEach(func() {
	if skipped {
		Skip("$RUN_CESSNA_TESTS not set; skipping")
	}

	fakeWorker = new(cessnafakes.FakeWorker)
	fakeGardenClient = new(gardenfakes.FakeClient)
	fakeBaggageClaimClient = new(baggageclaimfakes.FakeClient)

	fakeWorker.BaggageClaimClientReturns(fakeBaggageClaimClient)
	fakeWorker.GardenClientReturns(fakeGardenClient)

	workerIp, found = os.LookupEnv("WORKER_IP")
	Expect(found).To(BeTrue(), "Must set WORKER_IP")

	tarPath, found = os.LookupEnv("ROOTFS_TAR_PATH")
	Expect(found).To(BeTrue(), "Must set ROOTFS_TAR_PATH")

	tarURL, found = os.LookupEnv("ROOTFS_TAR_URL")
	Expect(found).To(BeTrue(), "Must set ROOTFS_TAR_URL")

	worker = NewWorker(fmt.Sprintf("%s:7777", workerIp), fmt.Sprintf("http://%s:7788", workerIp))
	logger = lagertest.NewTestLogger("logger-test")
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

	handle, err := uuid.NewV4()
	if err != nil {
		return "", err
	}

	volume, err := baggageclaimClient.CreateVolume(
		lager.NewLogger("create-volume-for-base-resource"),
		handle.String(),
		volumeSpec,
	)

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
