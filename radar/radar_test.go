package radar_test

import (
	"errors"
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"

	. "github.com/concourse/atc/radar"
	"github.com/concourse/atc/radar/fakes"
	"github.com/concourse/atc/resource"
	rfakes "github.com/concourse/atc/resource/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Radar", func() {
	var (
		fakeTracker   *rfakes.FakeTracker
		fakeVersionDB *fakes.FakeVersionDB
		configDB      *dbfakes.FakeConfigDB
		interval      time.Duration

		radar *Radar

		resourceConfig atc.ResourceConfig

		locker               *fakes.FakeLocker
		readLock             *dbfakes.FakeLock
		writeLock            *dbfakes.FakeLock
		writeImmediatelyLock *dbfakes.FakeLock

		process ifrit.Process
	)

	BeforeEach(func() {
		fakeTracker = new(rfakes.FakeTracker)
		fakeVersionDB = new(fakes.FakeVersionDB)
		locker = new(fakes.FakeLocker)
		configDB = new(dbfakes.FakeConfigDB)
		interval = 100 * time.Millisecond

		radar = NewRadar(fakeTracker, fakeVersionDB, interval, locker, configDB)

		resourceConfig = atc.ResourceConfig{
			Name:   "some-resource",
			Type:   "git",
			Source: atc.Source{"uri": "http://example.com"},
		}

		configDB.GetConfigReturns(atc.Config{
			Resources: atc.ResourceConfigs{
				resourceConfig,
			},
		}, 1, nil)

		fakeVersionDB.GetResourceReturns(db.Resource{
			Name:   "some-resource",
			Paused: false,
		}, nil)

		readLock = new(dbfakes.FakeLock)
		locker.AcquireReadLockReturns(readLock, nil)

		writeLock = new(dbfakes.FakeLock)
		locker.AcquireWriteLockReturns(writeLock, nil)

		writeImmediatelyLock = new(dbfakes.FakeLock)
		locker.AcquireWriteLockImmediatelyReturns(writeImmediatelyLock, nil)
	})

	Describe("Scanner", func() {
		var (
			fakeResource *rfakes.FakeResource

			times chan time.Time
		)

		BeforeEach(func() {
			fakeResource = new(rfakes.FakeResource)
			fakeTracker.InitReturns(fakeResource, nil)

			times = make(chan time.Time, 100)

			fakeResource.CheckStub = func(atc.Source, atc.Version) ([]atc.Version, error) {
				times <- time.Now()
				return nil, nil
			}
		})

		JustBeforeEach(func() {
			process = ifrit.Invoke(radar.Scanner(lagertest.NewTestLogger("test"), "some-resource"))
		})

		AfterEach(func() {
			process.Signal(os.Interrupt)
			<-process.Wait()
		})

		It("constructs the resource of the correct type", func() {
			Eventually(times).Should(Receive())

			sessionID, typ := fakeTracker.InitArgsForCall(0)
			Ω(sessionID).Should(Equal(resource.Session{
				ID: worker.Identifier{
					Name: "some-resource",
					Type: "check",

					CheckType:   "git",
					CheckSource: resourceConfig.Source,
				},
				Ephemeral: true,
			}))
			Ω(typ).Should(Equal(resource.ResourceType("git")))
		})

		It("checks on a specified interval", func() {
			var time1 time.Time
			var time2 time.Time

			Eventually(times).Should(Receive(&time1))
			Eventually(times).Should(Receive(&time2))

			Ω(time2.Sub(time1)).Should(BeNumerically("~", interval, interval/4))
		})

		It("grabs a resource checking lock before checking, releases after done", func() {
			Eventually(times).Should(Receive())

			Ω(locker.AcquireWriteLockImmediatelyCallCount()).Should(Equal(1))

			lockedInputs := locker.AcquireWriteLockImmediatelyArgsForCall(0)
			Ω(lockedInputs).Should(Equal([]db.NamedLock{db.ResourceCheckingLock("some-resource")}))

			Ω(writeImmediatelyLock.ReleaseCallCount()).Should(Equal(1))
		})

		It("releases after checking", func() {
			Eventually(times).Should(Receive())

			Ω(fakeResource.ReleaseCallCount()).Should(Equal(1))
		})

		Context("when there is no current version", func() {
			It("checks from nil", func() {
				Eventually(times).Should(Receive())

				_, version := fakeResource.CheckArgsForCall(0)
				Ω(version).Should(BeNil())
			})
		})

		Context("when there is a current version", func() {
			BeforeEach(func() {
				fakeVersionDB.GetLatestVersionedResourceReturns(db.SavedVersionedResource{
					ID:                1,
					VersionedResource: db.VersionedResource{Version: db.Version{"version": "1"}},
				}, nil)
			})

			It("checks from it", func() {
				Eventually(times).Should(Receive())

				_, version := fakeResource.CheckArgsForCall(0)
				Ω(version).Should(Equal(atc.Version{"version": "1"}))

				fakeVersionDB.GetLatestVersionedResourceReturns(db.SavedVersionedResource{
					ID:                2,
					VersionedResource: db.VersionedResource{Version: db.Version{"version": "2"}},
				}, nil)

				Eventually(times).Should(Receive())

				_, version = fakeResource.CheckArgsForCall(1)
				Ω(version).Should(Equal(atc.Version{"version": "2"}))
			})
		})

		Context("when the check returns versions", func() {
			var checkedFrom chan atc.Version

			var nextVersions []atc.Version

			BeforeEach(func() {
				checkedFrom = make(chan atc.Version, 100)

				nextVersions = []atc.Version{
					{"version": "1"},
					{"version": "2"},
					{"version": "3"},
				}

				checkResults := map[int][]atc.Version{
					0: nextVersions,
				}

				check := 0
				fakeResource.CheckStub = func(source atc.Source, from atc.Version) ([]atc.Version, error) {
					defer GinkgoRecover()

					Ω(source).Should(Equal(resourceConfig.Source))

					checkedFrom <- from
					result := checkResults[check]
					check++

					return result, nil
				}
			})

			It("saves them all, in order", func() {
				Eventually(fakeVersionDB.SaveResourceVersionsCallCount).Should(Equal(1))

				resourceConfig, versions := fakeVersionDB.SaveResourceVersionsArgsForCall(0)
				Ω(resourceConfig).Should(Equal(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "git",
					Source: atc.Source{"uri": "http://example.com"},
				}))
				Ω(versions).Should(Equal([]atc.Version{
					{"version": "1"},
					{"version": "2"},
					{"version": "3"},
				}))
			})
		})

		Context("when checking fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeResource.CheckReturns(nil, disaster)
			})

			It("exits with the failure", func() {
				Eventually(process.Wait()).Should(Receive(Equal(disaster)))
			})
		})

		Context("when the resource is paused", func() {
			BeforeEach(func() {
				fakeVersionDB.GetResourceReturns(db.Resource{
					Name:   "some-resource",
					Paused: true,
				}, nil)
			})

			It("exits the process", func() {
				Consistently(times, 500*time.Millisecond).ShouldNot(Receive())
			})
		})

		Context("when checking if the resource is paused fails", func() {
			disaster := errors.New("disaster")

			BeforeEach(func() {
				fakeVersionDB.GetResourceReturns(db.Resource{}, disaster)
			})

			It("exits the process", func() {
				Eventually(process.Wait()).Should(Receive(Equal(disaster)))
			})
		})

		Context("when the config changes", func() {
			var newConfig atc.Config

			BeforeEach(func() {
				configs := make(chan atc.Config, 1)
				configs <- atc.Config{
					Resources: atc.ResourceConfigs{resourceConfig},
				}

				configDB.GetConfigStub = func() (atc.Config, db.ConfigID, error) {
					select {
					case c := <-configs:
						return c, 1, nil
					default:
						return newConfig, 2, nil
					}
				}
			})

			Context("with new configuration for the resource", func() {
				var newResource atc.ResourceConfig

				BeforeEach(func() {
					newResource = atc.ResourceConfig{
						Name:   "some-resource",
						Type:   "git",
						Source: atc.Source{"uri": "http://example.com/updated-uri"},
					}

					newConfig = atc.Config{
						Resources: atc.ResourceConfigs{newResource},
					}
				})

				It("checks using the new config", func() {
					Eventually(times).Should(Receive())

					source, _ := fakeResource.CheckArgsForCall(0)
					Ω(source).Should(Equal(resourceConfig.Source))

					Eventually(times).Should(Receive())

					source, _ = fakeResource.CheckArgsForCall(1)
					Ω(source).Should(Equal(atc.Source{"uri": "http://example.com/updated-uri"}))
				})
			})

			Context("with the resource removed", func() {
				BeforeEach(func() {
					newConfig = atc.Config{
						Resources: atc.ResourceConfigs{},
					}
				})

				It("exits", func() {
					Eventually(times).Should(Receive())

					source, _ := fakeResource.CheckArgsForCall(0)
					Ω(source).Should(Equal(resourceConfig.Source))

					Eventually(process.Wait()).Should(Receive())
				})
			})
		})

		Context("and checking takes a while", func() {
			BeforeEach(func() {
				checked := false

				fakeResource.CheckStub = func(atc.Source, atc.Version) ([]atc.Version, error) {
					times <- time.Now()

					if checked {
						time.Sleep(interval)
					}

					checked = true

					return nil, nil
				}
			})

			It("does not count it towards the interval", func() {
				var time1 time.Time
				var time2 time.Time

				Eventually(times).Should(Receive(&time1))
				Eventually(times, 2).Should(Receive(&time2))

				Ω(time2.Sub(time1)).Should(BeNumerically("~", interval, interval/2))
			})
		})
	})

	Describe("Scan", func() {
		var (
			fakeResource *rfakes.FakeResource

			scanErr error
		)

		BeforeEach(func() {
			fakeResource = new(rfakes.FakeResource)
			fakeTracker.InitReturns(fakeResource, nil)
		})

		JustBeforeEach(func() {
			scanErr = radar.Scan(lagertest.NewTestLogger("test"), "some-resource")
		})

		It("succeeds", func() {
			Ω(scanErr).ShouldNot(HaveOccurred())
		})

		It("constructs the resource of the correct type", func() {
			sessionID, typ := fakeTracker.InitArgsForCall(0)
			Ω(sessionID).Should(Equal(resource.Session{
				ID: worker.Identifier{
					Name: "some-resource",
					Type: "check",

					CheckType:   "git",
					CheckSource: resourceConfig.Source,
				},
				Ephemeral: true,
			}))
			Ω(typ).Should(Equal(resource.ResourceType("git")))
		})

		It("grabs a resource checking lock before checking, releases after done", func() {
			Ω(locker.AcquireWriteLockCallCount()).Should(Equal(1))

			lockedInputs := locker.AcquireWriteLockArgsForCall(0)
			Ω(lockedInputs).Should(Equal([]db.NamedLock{db.ResourceCheckingLock("some-resource")}))

			Ω(writeLock.ReleaseCallCount()).Should(Equal(1))
		})

		It("releases the resource", func() {
			Ω(fakeResource.ReleaseCallCount()).Should(Equal(1))
		})

		It("clears the resource's check error", func() {
			Ω(fakeVersionDB.SetResourceCheckErrorCallCount()).Should(Equal(1))

			resourceName, err := fakeVersionDB.SetResourceCheckErrorArgsForCall(0)
			Ω(resourceName).Should(Equal("some-resource"))
			Ω(err).Should(BeNil())
		})

		Context("when there is no current version", func() {
			It("checks from nil", func() {
				_, version := fakeResource.CheckArgsForCall(0)
				Ω(version).Should(BeNil())
			})
		})

		Context("when there is a current version", func() {
			BeforeEach(func() {
				fakeVersionDB.GetLatestVersionedResourceReturns(db.SavedVersionedResource{
					ID:                1,
					VersionedResource: db.VersionedResource{Version: db.Version{"version": "1"}},
				}, nil)
			})

			It("checks from it", func() {
				_, version := fakeResource.CheckArgsForCall(0)
				Ω(version).Should(Equal(atc.Version{"version": "1"}))
			})
		})

		Context("when the check returns versions", func() {
			var checkedFrom chan atc.Version

			var nextVersions []atc.Version

			BeforeEach(func() {
				checkedFrom = make(chan atc.Version, 100)

				nextVersions = []atc.Version{
					{"version": "1"},
					{"version": "2"},
					{"version": "3"},
				}

				checkResults := map[int][]atc.Version{
					0: nextVersions,
				}

				check := 0
				fakeResource.CheckStub = func(source atc.Source, from atc.Version) ([]atc.Version, error) {
					defer GinkgoRecover()

					Ω(source).Should(Equal(resourceConfig.Source))

					checkedFrom <- from
					result := checkResults[check]
					check++

					return result, nil
				}
			})

			It("saves them all, in order", func() {
				Ω(fakeVersionDB.SaveResourceVersionsCallCount()).Should(Equal(1))

				resourceConfig, versions := fakeVersionDB.SaveResourceVersionsArgsForCall(0)
				Ω(resourceConfig).Should(Equal(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "git",
					Source: atc.Source{"uri": "http://example.com"},
				}))
				Ω(versions).Should(Equal([]atc.Version{
					{"version": "1"},
					{"version": "2"},
					{"version": "3"},
				}))
			})
		})

		Context("when checking fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeResource.CheckReturns(nil, disaster)
			})

			It("returns the error", func() {
				Ω(scanErr).Should(Equal(disaster))
			})

			It("sets the resource's check error", func() {
				Ω(fakeVersionDB.SetResourceCheckErrorCallCount()).Should(Equal(1))

				resourceName, err := fakeVersionDB.SetResourceCheckErrorArgsForCall(0)
				Ω(resourceName).Should(Equal("some-resource"))
				Ω(err).Should(Equal(disaster))
			})
		})
	})
})
