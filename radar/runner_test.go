package radar_test

import (
	"context"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	. "github.com/concourse/atc/radar"
	"github.com/concourse/atc/radar/radarfakes"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Runner", func() {
	var (
		fakePipeline      *dbfakes.FakePipeline
		scanRunnerFactory *radarfakes.FakeScanRunnerFactory
		noop              bool
		syncInterval      time.Duration

		process                ifrit.Process
		fakeResourceRunner     *radarfakes.FakeIntervalRunner
		fakeResourceTypeRunner *radarfakes.FakeIntervalRunner
		fakeContext            context.Context
		fakeCancel             context.CancelFunc

		fakeResource1 *dbfakes.FakeResource
		fakeResource2 *dbfakes.FakeResource
	)

	BeforeEach(func() {
		scanRunnerFactory = new(radarfakes.FakeScanRunnerFactory)
		fakePipeline = new(dbfakes.FakePipeline)
		noop = false
		syncInterval = 100 * time.Millisecond

		fakeResource1 = new(dbfakes.FakeResource)
		fakeResource1.NameReturns("some-resource")
		fakeResource2 = new(dbfakes.FakeResource)
		fakeResource2.NameReturns("some-other-resource")
		fakePipeline.ResourcesReturns(db.Resources{fakeResource1, fakeResource2}, nil)

		fakeResourceType1 := new(dbfakes.FakeResourceType)
		fakeResourceType1.NameReturns("some-resource")
		fakeResourceType2 := new(dbfakes.FakeResourceType)
		fakeResourceType2.NameReturns("some-other-resource")
		fakePipeline.ResourceTypesReturns(db.ResourceTypes{fakeResourceType1, fakeResourceType2}, nil)

		fakePipeline.ScopedNameStub = func(thing string) string {
			return "pipeline:" + thing
		}

		fakeResourceRunner = new(radarfakes.FakeIntervalRunner)
		scanRunnerFactory.ScanResourceRunnerReturns(fakeResourceRunner)
		fakeResourceTypeRunner = new(radarfakes.FakeIntervalRunner)
		scanRunnerFactory.ScanResourceTypeRunnerReturns(fakeResourceTypeRunner)

		fcon, fcanc := context.WithCancel(context.Background())
		fakeContext = fcon
		fakeCancel = fcanc

		fakeResourceRunner.RunStub = func(ctx context.Context) error {
			<-fcon.Done()
			return nil
		}

		fakeResourceTypeRunner.RunStub = func(ctx context.Context) error {
			<-fcon.Done()
			return nil
		}
	})

	JustBeforeEach(func() {
		process = ginkgomon.Invoke(NewRunner(
			lagertest.NewTestLogger("test"),
			noop,
			scanRunnerFactory,
			fakePipeline,
			syncInterval,
		))
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
	})

	It("scans for every configured resource", func() {
		Eventually(scanRunnerFactory.ScanResourceRunnerCallCount).Should(Equal(2))

		_, call1Resource := scanRunnerFactory.ScanResourceRunnerArgsForCall(0)
		_, call2Resource := scanRunnerFactory.ScanResourceRunnerArgsForCall(1)

		resources := []string{call1Resource, call2Resource}
		Expect(resources).To(ConsistOf([]string{"some-resource", "some-other-resource"}))
	})

	Context("when new resources are configured", func() {
		It("scans for them eventually", func() {
			Eventually(scanRunnerFactory.ScanResourceRunnerCallCount).Should(Equal(2))

			_, call1Resource := scanRunnerFactory.ScanResourceRunnerArgsForCall(0)
			_, call2Resource := scanRunnerFactory.ScanResourceRunnerArgsForCall(1)
			resources := []string{call1Resource, call2Resource}
			Expect(resources).To(ConsistOf([]string{"some-resource", "some-other-resource"}))

			fakeResource3 := new(dbfakes.FakeResource)
			fakeResource3.NameReturns("another-resource")
			fakePipeline.ResourcesReturns(db.Resources{fakeResource1, fakeResource2, fakeResource3}, nil)

			Eventually(scanRunnerFactory.ScanResourceRunnerCallCount, time.Second).Should(Equal(3))

			_, call3Resource := scanRunnerFactory.ScanResourceRunnerArgsForCall(2)
			resources = append(resources, call3Resource)
			Expect(resources).To(ConsistOf([]string{"some-resource", "some-other-resource", "another-resource"}))

			Consistently(scanRunnerFactory.ScanResourceRunnerCallCount).Should(Equal(3))
		})
	})

	Context("when resources stop being able to check", func() {
		It("starts scanning again eventually", func() {
			Eventually(scanRunnerFactory.ScanResourceRunnerCallCount).Should(Equal(2))

			_, call1Resource := scanRunnerFactory.ScanResourceRunnerArgsForCall(0)
			_, call2Resource := scanRunnerFactory.ScanResourceRunnerArgsForCall(1)
			resources := []string{call1Resource, call2Resource}

			Expect(resources).To(ConsistOf([]string{"some-resource", "some-other-resource"}))

			fakeCancel()

			Eventually(scanRunnerFactory.ScanResourceRunnerCallCount, 10*syncInterval).Should(Equal(4))

			_, call3Resource := scanRunnerFactory.ScanResourceRunnerArgsForCall(2)
			_, call4Resource := scanRunnerFactory.ScanResourceRunnerArgsForCall(3)
			resources = append(resources, call3Resource, call4Resource)
			Expect(resources).To(ConsistOf([]string{"some-resource", "some-other-resource", "some-resource", "some-other-resource"}))

		})
	})

	Context("when resource types stop being able to check", func() {
		It("starts scanning again eventually", func() {
			Eventually(scanRunnerFactory.ScanResourceTypeRunnerCallCount).Should(Equal(2))

			_, call1Resource := scanRunnerFactory.ScanResourceTypeRunnerArgsForCall(0)
			_, call2Resource := scanRunnerFactory.ScanResourceTypeRunnerArgsForCall(1)
			resources := []string{call1Resource, call2Resource}

			fakeCancel()

			Eventually(scanRunnerFactory.ScanResourceTypeRunnerCallCount, 10*syncInterval).Should(Equal(4))

			_, call3Resource := scanRunnerFactory.ScanResourceTypeRunnerArgsForCall(2)
			_, call4Resource := scanRunnerFactory.ScanResourceTypeRunnerArgsForCall(3)
			resources = append(resources, call3Resource, call4Resource)
			Expect(resources).To(ConsistOf([]string{"some-resource", "some-other-resource", "some-resource", "some-other-resource"}))
		})
	})

	Context("when in noop mode", func() {
		BeforeEach(func() {
			noop = true
		})

		It("does not start scanning resources", func() {
			Expect(scanRunnerFactory.ScanResourceRunnerCallCount()).To(Equal(0))
			Expect(scanRunnerFactory.ScanResourceTypeRunnerCallCount()).To(Equal(0))
		})
	})
})
