package radar_test

import (
	"context"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
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

		config atc.Config

		process                ifrit.Process
		fakeResourceRunner     *radarfakes.FakeIntervalRunner
		fakeResourceTypeRunner *radarfakes.FakeIntervalRunner
		fakeContext            context.Context
		fakeCancel             context.CancelFunc
	)

	BeforeEach(func() {
		scanRunnerFactory = new(radarfakes.FakeScanRunnerFactory)
		fakePipeline = new(dbfakes.FakePipeline)
		noop = false
		syncInterval = 100 * time.Millisecond

		config = atc.Config{
			Resources: atc.ResourceConfigs{
				{
					Name: "some-resource",
				},
				{
					Name: "some-other-resource",
				},
			},
			ResourceTypes: atc.ResourceTypes{
				{
					Name: "some-resource",
				},
				{
					Name: "some-other-resource",
				},
			},
		}

		fakePipeline.ScopedNameStub = func(thing string) string {
			return "pipeline:" + thing
		}
		fakePipeline.ReloadReturns(true, nil)
		fakePipeline.ConfigStub = func() (atc.Config, atc.RawConfig, db.ConfigVersion, error) {
			return config, "", 0, nil
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

			config.Resources = append(config.Resources, atc.ResourceConfig{
				Name: "another-resource",
			})

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

			_, call1Resource := scanRunnerFactory.ScanResourceRunnerArgsForCall(0)
			_, call2Resource := scanRunnerFactory.ScanResourceRunnerArgsForCall(1)
			resources := []string{call1Resource, call2Resource}

			fakeCancel()

			Eventually(scanRunnerFactory.ScanResourceTypeRunnerCallCount, 10*syncInterval).Should(Equal(4))

			_, call3Resource := scanRunnerFactory.ScanResourceRunnerArgsForCall(2)
			_, call4Resource := scanRunnerFactory.ScanResourceRunnerArgsForCall(3)
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
