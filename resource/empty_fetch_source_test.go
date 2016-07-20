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

var _ = Describe("EmptyFetchSource", func() {
	var (
		fetchSource FetchSource

		fakeContainer        *workerfakes.FakeContainer
		fakeContainerCreator *resourcefakes.FakeFetchContainerCreator
		resourceOptions      *resourcefakes.FakeResourceOptions
		fakeVolume           *workerfakes.FakeVolume
		fakeWorker           *workerfakes.FakeWorker
		cacheID              *resourcefakes.FakeCacheIdentifier

		signals <-chan os.Signal
		ready   chan<- struct{}
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
		fakeWorker = new(workerfakes.FakeWorker)
		fakeContainerCreator = new(resourcefakes.FakeFetchContainerCreator)
		fakeContainerCreator.CreateWithVolumeReturns(fakeContainer, nil)
		cacheID = new(resourcefakes.FakeCacheIdentifier)

		fetchSource = NewEmptyFetchSource(
			logger,
			fakeWorker,
			cacheID,
			fakeContainerCreator,
			resourceOptions,
		)

		cacheID.FindOnReturns(fakeVolume, true, nil)
	})

	Describe("Initialize", func() {
		var initErr error

		BeforeEach(func() {
			resourceOptions.ResourceTypeReturns(ResourceType("fake-resource-type"))
		})

		JustBeforeEach(func() {
			initErr = fetchSource.Initialize(signals, ready)
		})

		It("creates container with volume and worker", func() {
			Expect(initErr).NotTo(HaveOccurred())
			Expect(fakeContainerCreator.CreateWithVolumeCallCount()).To(Equal(1))
			resourceType, volume, worker := fakeContainerCreator.CreateWithVolumeArgsForCall(0)
			Expect(resourceType).To(Equal("fake-resource-type"))
			Expect(volume).To(Equal(fakeVolume))
			Expect(worker).To(Equal(fakeWorker))
		})

		It("fetches versioned source", func() {
			Expect(initErr).NotTo(HaveOccurred())
			Expect(fakeContainer.RunCallCount()).To(Equal(1))
		})

		It("initializes cache", func() {
			Expect(initErr).NotTo(HaveOccurred())
			Expect(fakeVolume.SetPropertyCallCount()).To(Equal(1))
		})

		Context("when getting resource fails with ErrAborted", func() {
			BeforeEach(func() {
				fakeContainer.RunReturns(nil, ErrAborted)
			})

			It("returns ErrInterrupted", func() {
				Expect(initErr).To(HaveOccurred())
				Expect(initErr).To(Equal(ErrInterrupted))
			})
		})

		Context("when getting resource fails with other error", func() {
			var disaster error

			BeforeEach(func() {
				disaster = errors.New("failed")
				fakeContainer.RunReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(initErr).To(HaveOccurred())
				Expect(initErr).To(Equal(disaster))
			})
		})

		Context("when volume is not found", func() {
			BeforeEach(func() {
				cacheID.FindOnReturns(nil, false, nil)
			})

			Context("when creating cache volume succeeds", func() {
				var newVolume *workerfakes.FakeVolume

				BeforeEach(func() {
					newVolume = new(workerfakes.FakeVolume)
					cacheID.CreateOnReturns(newVolume, nil)
				})

				It("creates volume", func() {
					Expect(initErr).NotTo(HaveOccurred())
					Expect(cacheID.CreateOnCallCount()).To(Equal(1))
					_, worker := cacheID.CreateOnArgsForCall(0)
					Expect(worker).To(Equal(fakeWorker))
				})

				It("uses newly created cache volume", func() {
					Expect(initErr).NotTo(HaveOccurred())
					Expect(fakeContainerCreator.CreateWithVolumeCallCount()).To(Equal(1))
					resourceType, volume, worker := fakeContainerCreator.CreateWithVolumeArgsForCall(0)
					Expect(resourceType).To(Equal("fake-resource-type"))
					Expect(volume).To(Equal(newVolume))
					Expect(worker).To(Equal(fakeWorker))
				})
			})

			Context("when creating volume fails", func() {
				var disaster error

				BeforeEach(func() {
					disaster = errors.New("failed")
					cacheID.CreateOnReturns(nil, disaster)
				})

				It("returns an error", func() {
					Expect(initErr).To(HaveOccurred())
					Expect(initErr).To(Equal(disaster))
				})
			})
		})
	})

	Describe("Release", func() {
		Context("when initialized", func() {
			BeforeEach(func() {
				err := fetchSource.Initialize(signals, ready)
				Expect(err).NotTo(HaveOccurred())
			})

			It("releases volume", func() {
				finalTTL := worker.FinalTTL(5 * time.Second)
				fetchSource.Release(finalTTL)
				Expect(fakeVolume.ReleaseCallCount()).To(Equal(1))
				ttl := fakeVolume.ReleaseArgsForCall(0)
				Expect(ttl).To(Equal(finalTTL))
			})

			It("releases container", func() {
				finalTTL := worker.FinalTTL(5 * time.Second)
				fetchSource.Release(finalTTL)
				Expect(fakeContainer.ReleaseCallCount()).To(Equal(1))
				ttl := fakeContainer.ReleaseArgsForCall(0)
				Expect(ttl).To(Equal(finalTTL))
			})
		})
	})
})
