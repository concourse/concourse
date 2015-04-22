package radar_test

import (
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"
	. "github.com/concourse/atc/radar"
	"github.com/concourse/atc/radar/fakes"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Runner", func() {
	var (
		locker         *fakes.FakeLocker
		configDB       *dbfakes.FakeConfigDB
		scannerFactory *fakes.FakeScannerFactory
		noop           bool
		syncInterval   time.Duration

		initialConfig atc.Config

		process ifrit.Process
	)

	BeforeEach(func() {
		locker = new(fakes.FakeLocker)
		scannerFactory = new(fakes.FakeScannerFactory)
		configDB = new(dbfakes.FakeConfigDB)
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

		configDB.GetConfigReturns(initialConfig, 1, nil)

		scannerFactory.ScannerStub = func(lager.Logger, string) ifrit.Runner {
			return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
				close(ready)
				<-signals
				return nil
			})
		}
	})

	JustBeforeEach(func() {
		process = ginkgomon.Invoke(NewRunner(
			lagertest.NewTestLogger("test"),
			noop,
			locker,
			scannerFactory,
			configDB,
			syncInterval,
		))
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
	})

	It("scans for every configured resource", func() {
		Eventually(scannerFactory.ScannerCallCount).Should(Equal(2))

		_, resource := scannerFactory.ScannerArgsForCall(0)
		Ω(resource).Should(Equal("some-resource"))

		_, resource = scannerFactory.ScannerArgsForCall(1)
		Ω(resource).Should(Equal("some-other-resource"))
	})

	Context("when new resources are configured", func() {
		var updateConfig chan<- atc.Config

		BeforeEach(func() {
			configs := make(chan atc.Config)
			updateConfig = configs

			config := initialConfig

			configDB.GetConfigStub = func(string) (atc.Config, db.ConfigVersion, error) {
				select {
				case config = <-configs:
				default:
				}

				return config, 1, nil
			}
		})

		It("scans for them eventually", func() {
			Eventually(scannerFactory.ScannerCallCount).Should(Equal(2))

			_, resource := scannerFactory.ScannerArgsForCall(0)
			Ω(resource).Should(Equal("some-resource"))

			_, resource = scannerFactory.ScannerArgsForCall(1)
			Ω(resource).Should(Equal("some-other-resource"))

			newConfig := initialConfig
			newConfig.Resources = append(newConfig.Resources, atc.ResourceConfig{
				Name: "another-resource",
			})

			updateConfig <- newConfig

			Eventually(scannerFactory.ScannerCallCount).Should(Equal(3))

			_, resource = scannerFactory.ScannerArgsForCall(2)
			Ω(resource).Should(Equal("another-resource"))

			Consistently(scannerFactory.ScannerCallCount).Should(Equal(3))
		})
	})

	Context("when resources stop being able to check", func() {
		var scannerExit chan struct{}

		BeforeEach(func() {
			scannerExit = make(chan struct{})

			scannerFactory.ScannerStub = func(lager.Logger, string) ifrit.Runner {
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
			Eventually(scannerFactory.ScannerCallCount).Should(Equal(2))

			_, resource := scannerFactory.ScannerArgsForCall(0)
			Ω(resource).Should(Equal("some-resource"))

			_, resource = scannerFactory.ScannerArgsForCall(1)
			Ω(resource).Should(Equal("some-other-resource"))

			close(scannerExit)

			Eventually(scannerFactory.ScannerCallCount, 10*syncInterval).Should(Equal(4))

			_, resource = scannerFactory.ScannerArgsForCall(2)
			Ω(resource).Should(Equal("some-resource"))

			_, resource = scannerFactory.ScannerArgsForCall(3)
			Ω(resource).Should(Equal("some-other-resource"))
		})
	})

	Context("when in noop mode", func() {
		BeforeEach(func() {
			noop = true
		})

		It("does not start scanning resources", func() {
			Ω(scannerFactory.ScannerCallCount()).Should(Equal(0))
		})
	})
})
