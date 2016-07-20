package resource_test

import (
	"errors"
	"os"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	gfakes "github.com/cloudfoundry-incubator/garden/fakes"
	. "github.com/concourse/atc/resource"
	"github.com/concourse/atc/resource/resourcefakes"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/workerfakes"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContainerFetchSource", func() {
	var (
		fetchSource     FetchSource
		fakeContainer   *workerfakes.FakeContainer
		resourceOptions ResourceOptions
		signals         <-chan os.Signal
		ready           chan<- struct{}
		fakeVolume      *workerfakes.FakeVolume
	)

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("test")
		fakeContainer = new(workerfakes.FakeContainer)
		resourceOptions = new(resourcefakes.FakeResourceOptions)
		signals = make(<-chan os.Signal)
		ready = make(chan<- struct{})

		fakeContainer.PropertyReturns("", errors.New("nope"))
		inProcess := new(gfakes.FakeProcess)
		inProcess.IDReturns("process-id")
		inProcess.WaitStub = func() (int, error) {
			return 0, nil
		}

		fakeContainer.RunStub = func(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
			_, err := io.Stdout.Write([]byte("{}"))
			Expect(err).NotTo(HaveOccurred())

			return inProcess, nil
		}

		fakeVolume = new(workerfakes.FakeVolume)

		fakeContainer.VolumeMountsReturns([]worker.VolumeMount{
			worker.VolumeMount{
				Volume:    fakeVolume,
				MountPath: "/tmp/build/get",
			},
		})

		fetchSource = NewContainerFetchSource(
			logger,
			fakeContainer,
			resourceOptions,
		)
	})

	Describe("Initialize", func() {
		It("fetches versioned source", func() {
			err := fetchSource.Initialize(signals, ready)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeContainer.RunCallCount()).To(Equal(1))
		})

		It("initializes cache", func() {
			err := fetchSource.Initialize(signals, ready)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeVolume.SetPropertyCallCount()).To(Equal(1))
		})

		Context("when getting resource fails with ErrAborted", func() {
			BeforeEach(func() {
				fakeContainer.RunReturns(nil, ErrAborted)
			})

			It("returns ErrInterrupted", func() {
				err := fetchSource.Initialize(signals, ready)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(ErrInterrupted))
			})
		})

		Context("when getting resource fails with other error", func() {
			var disaster error

			BeforeEach(func() {
				disaster = errors.New("failed")
				fakeContainer.RunReturns(nil, disaster)
			})

			It("returns the error", func() {
				err := fetchSource.Initialize(signals, ready)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(disaster))
			})
		})
	})

	Describe("Release", func() {
		It("releases container", func() {
			finalTTL := worker.FinalTTL(5 * time.Second)
			fetchSource.Release(finalTTL)
			Expect(fakeContainer.ReleaseCallCount()).To(Equal(1))
			ttl := fakeContainer.ReleaseArgsForCall(0)
			Expect(ttl).To(Equal(finalTTL))
		})
	})
})
