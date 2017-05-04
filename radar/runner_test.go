package radar_test

import (
	"os"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
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
		pipelineDB        *dbfakes.FakePipelineDB
		scanRunnerFactory *radarfakes.FakeScanRunnerFactory
		noop              bool
		syncInterval      time.Duration

		initialConfig atc.Config

		process ifrit.Process
	)

	BeforeEach(func() {
		scanRunnerFactory = new(radarfakes.FakeScanRunnerFactory)
		pipelineDB = new(dbfakes.FakePipelineDB)
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
			ResourceTypes: atc.ResourceTypes{
				{
					Name: "some-resource",
				},
				{
					Name: "some-other-resource",
				},
			},
		}

		pipelineDB.ScopedNameStub = func(thing string) string {
			return "pipeline:" + thing
		}
		pipelineDB.ReloadReturns(true, nil)
		pipelineDB.ConfigReturns(initialConfig)

		scanRunnerFactory.ScanResourceRunnerStub = func(lager.Logger, string) ifrit.Runner {
			return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
				close(ready)
				<-signals
				return nil
			})
		}

		scanRunnerFactory.ScanResourceTypeRunnerStub = func(lager.Logger, string) ifrit.Runner {
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
			scanRunnerFactory,
			pipelineDB,
			syncInterval,
		))
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
	})

	It("scans for every configured resource", func() {
		Eventually(scanRunnerFactory.ScanResourceRunnerCallCount).Should(Equal(2))

		_, resource := scanRunnerFactory.ScanResourceRunnerArgsForCall(0)
		Expect(resource).To(Equal("some-resource"))

		_, resource = scanRunnerFactory.ScanResourceRunnerArgsForCall(1)
		Expect(resource).To(Equal("some-other-resource"))
	})

	Context("when new resources are configured", func() {
		var updateConfig chan<- atc.Config

		BeforeEach(func() {
			configs := make(chan atc.Config)
			updateConfig = configs

			config := initialConfig

			pipelineDB.ConfigStub = func() atc.Config {
				select {
				case config = <-configs:
				default:
				}

				return config
			}
		})

		It("scans for them eventually", func() {
			Eventually(scanRunnerFactory.ScanResourceRunnerCallCount).Should(Equal(2))

			_, resource := scanRunnerFactory.ScanResourceRunnerArgsForCall(0)
			Expect(resource).To(Equal("some-resource"))

			_, resource = scanRunnerFactory.ScanResourceRunnerArgsForCall(1)
			Expect(resource).To(Equal("some-other-resource"))

			newConfig := initialConfig
			newConfig.Resources = append(newConfig.Resources, atc.ResourceConfig{
				Name: "another-resource",
			})

			updateConfig <- newConfig

			Eventually(scanRunnerFactory.ScanResourceRunnerCallCount).Should(Equal(3))

			_, resource = scanRunnerFactory.ScanResourceRunnerArgsForCall(2)
			Expect(resource).To(Equal("another-resource"))

			Consistently(scanRunnerFactory.ScanResourceRunnerCallCount).Should(Equal(3))
		})
	})

	Context("when resources stop being able to check", func() {
		var scannerExit chan struct{}

		BeforeEach(func() {
			scannerExit = make(chan struct{})

			scanRunnerFactory.ScanResourceRunnerStub = func(lager.Logger, string) ifrit.Runner {
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
			Eventually(scanRunnerFactory.ScanResourceRunnerCallCount).Should(Equal(2))

			_, resource := scanRunnerFactory.ScanResourceRunnerArgsForCall(0)
			Expect(resource).To(Equal("some-resource"))

			_, resource = scanRunnerFactory.ScanResourceRunnerArgsForCall(1)
			Expect(resource).To(Equal("some-other-resource"))

			close(scannerExit)

			Eventually(scanRunnerFactory.ScanResourceRunnerCallCount, 10*syncInterval).Should(Equal(4))

			_, resource = scanRunnerFactory.ScanResourceRunnerArgsForCall(2)
			Expect(resource).To(Equal("some-resource"))

			_, resource = scanRunnerFactory.ScanResourceRunnerArgsForCall(3)
			Expect(resource).To(Equal("some-other-resource"))
		})
	})

	Context("when resource types stop being able to check", func() {
		var scannerExit chan struct{}

		BeforeEach(func() {
			scannerExit = make(chan struct{})

			scanRunnerFactory.ScanResourceTypeRunnerStub = func(lager.Logger, string) ifrit.Runner {
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
			Eventually(scanRunnerFactory.ScanResourceTypeRunnerCallCount).Should(Equal(2))

			_, resource := scanRunnerFactory.ScanResourceTypeRunnerArgsForCall(0)
			Expect(resource).To(Equal("some-resource"))

			_, resource = scanRunnerFactory.ScanResourceTypeRunnerArgsForCall(1)
			Expect(resource).To(Equal("some-other-resource"))

			close(scannerExit)

			Eventually(scanRunnerFactory.ScanResourceTypeRunnerCallCount, 10*syncInterval).Should(Equal(4))

			_, resource = scanRunnerFactory.ScanResourceTypeRunnerArgsForCall(2)
			Expect(resource).To(Equal("some-resource"))

			_, resource = scanRunnerFactory.ScanResourceTypeRunnerArgsForCall(3)
			Expect(resource).To(Equal("some-other-resource"))
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
