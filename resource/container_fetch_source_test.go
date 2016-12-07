package resource_test

import (
	"errors"
	"os"

	"code.cloudfoundry.org/garden"
	gfakes "code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/concourse/atc/resource"
	"github.com/concourse/atc/resource/resourcefakes"
	"github.com/concourse/atc/worker/workerfakes"

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

		fetchSource = NewContainerFetchSource(
			logger,
			fakeContainer,
			fakeVolume,
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
			Expect(fakeVolume.InitializeCallCount()).To(Equal(1))
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
			Expect(fakeContainer.ReleaseCallCount()).To(Equal(0))
			fetchSource.Release()
			Expect(fakeContainer.ReleaseCallCount()).To(Equal(1))
		})
	})
})
