package radar_test

import (
	"os"
	"time"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/radar"
	"github.com/concourse/atc/radar/fakes"
	"github.com/concourse/turbine"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"github.com/tedsuo/rata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Runner", func() {
	var (
		locker          *fakes.FakeLocker
		configDB        *fakes.FakeConfigDB
		scanner         *fakes.FakeScanner
		noop            bool
		turbineEndpoint *rata.RequestGenerator
		syncInterval    time.Duration

		initialConfig atc.Config

		process ifrit.Process
	)

	BeforeEach(func() {
		locker = new(fakes.FakeLocker)
		scanner = new(fakes.FakeScanner)
		configDB = new(fakes.FakeConfigDB)
		noop = false
		syncInterval = 100 * time.Millisecond

		initialConfig = atc.Config{
			Resources: atc.ResourceConfigs{
				{
					Name: "some-resource",
				},
				{
					Name: "some-other-resource",
				},
			},
		}

		configDB.GetConfigReturns(initialConfig, nil)

		scanner.ScanStub = func(ResourceChecker, string) ifrit.Runner {
			return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
				close(ready)
				<-signals
				return nil
			})
		}

		turbineEndpoint = rata.NewRequestGenerator("turbine-host", turbine.Routes)
	})

	JustBeforeEach(func() {
		process = ginkgomon.Invoke(NewRunner(
			lagertest.NewTestLogger("test"),
			noop,
			locker,
			scanner,
			configDB,
			syncInterval,
			turbineEndpoint,
		))
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
	})

	It("scans for every configured resource", func() {
		Eventually(scanner.ScanCallCount).Should(Equal(2))

		_, resource := scanner.ScanArgsForCall(0)
		Ω(resource).Should(Equal("some-resource"))

		_, resource = scanner.ScanArgsForCall(1)
		Ω(resource).Should(Equal("some-other-resource"))
	})

	Context("when new resources are configured", func() {
		var updateConfig chan<- atc.Config

		BeforeEach(func() {
			configs := make(chan atc.Config)
			updateConfig = configs

			config := initialConfig

			configDB.GetConfigStub = func() (atc.Config, error) {
				select {
				case config = <-configs:
				default:
				}

				return config, nil
			}
		})

		It("scans for them eventually", func() {
			Eventually(scanner.ScanCallCount).Should(Equal(2))

			_, resource := scanner.ScanArgsForCall(0)
			Ω(resource).Should(Equal("some-resource"))

			_, resource = scanner.ScanArgsForCall(1)
			Ω(resource).Should(Equal("some-other-resource"))

			newConfig := initialConfig
			newConfig.Resources = append(newConfig.Resources, atc.ResourceConfig{
				Name: "another-resource",
			})

			updateConfig <- newConfig

			Eventually(scanner.ScanCallCount).Should(Equal(3))

			_, resource = scanner.ScanArgsForCall(2)
			Ω(resource).Should(Equal("another-resource"))

			Consistently(scanner.ScanCallCount).Should(Equal(3))
		})
	})

	Context("when resources stop being able to check", func() {
		var scannerExit chan struct{}

		BeforeEach(func() {
			scannerExit = make(chan struct{})

			scanner.ScanStub = func(ResourceChecker, string) ifrit.Runner {
				return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
					close(ready)

					select {
					case <-signals:
						return nil
					case <-scannerExit:
						return nil
					}
				})
			}
		})

		It("starts scanning again eventually", func() {
			Eventually(scanner.ScanCallCount).Should(Equal(2))

			_, resource := scanner.ScanArgsForCall(0)
			Ω(resource).Should(Equal("some-resource"))

			_, resource = scanner.ScanArgsForCall(1)
			Ω(resource).Should(Equal("some-other-resource"))

			close(scannerExit)

			Eventually(scanner.ScanCallCount, 10*syncInterval).Should(Equal(4))

			_, resource = scanner.ScanArgsForCall(2)
			Ω(resource).Should(Equal("some-resource"))

			_, resource = scanner.ScanArgsForCall(3)
			Ω(resource).Should(Equal("some-other-resource"))
		})
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
