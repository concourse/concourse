package radar_test

import (
	"github.com/concourse/atc/config"
	. "github.com/concourse/atc/radar"
	"github.com/concourse/atc/radar/fakes"
	"github.com/concourse/turbine"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"github.com/tedsuo/rata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Runner", func() {
	var (
		locker          *fakes.FakeLocker
		scanner         *fakes.FakeScanner
		noop            bool
		resources       config.Resources
		turbineEndpoint *rata.RequestGenerator

		process ifrit.Process
	)

	BeforeEach(func() {
		locker = new(fakes.FakeLocker)
		scanner = new(fakes.FakeScanner)

		noop = false

		resources = config.Resources{
			{
				Name: "some-resource",
			},
			{
				Name: "some-other-resource",
			},
		}

		turbineEndpoint = rata.NewRequestGenerator("turbine-host", turbine.Routes)
	})

	JustBeforeEach(func() {
		process = ginkgomon.Invoke(&Runner{
			Locker:          locker,
			Scanner:         scanner,
			Noop:            noop,
			Resources:       resources,
			TurbineEndpoint: turbineEndpoint,
		})
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
	})

	It("scans for every given resource", func() {
		Eventually(scanner.ScanCallCount).Should(Equal(2))

		_, resource := scanner.ScanArgsForCall(0)
		Ω(resource).Should(Equal(config.Resource{Name: "some-resource"}))

		_, resource = scanner.ScanArgsForCall(1)
		Ω(resource).Should(Equal(config.Resource{Name: "some-other-resource"}))
	})

	Context("when in noop mode", func() {
		BeforeEach(func() {
			noop = true
		})

		It("does not start scanning resources", func() {
			Ω(scanner.ScanCallCount()).Should(Equal(0))
		})
	})
})
