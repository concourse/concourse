package radar_test

import (
	"errors"
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/clock/fakeclock"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"

	dbfakes "github.com/concourse/atc/db/fakes"
	. "github.com/concourse/atc/radar"
	"github.com/concourse/atc/radar/fakes"
	"github.com/concourse/atc/resource"
	rfakes "github.com/concourse/atc/resource/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Radar", func() {
	var (
		epoch time.Time

		fakeTracker *rfakes.FakeTracker
		fakeRadarDB *fakes.FakeRadarDB
		fakeClock   *fakeclock.FakeClock
		interval    time.Duration

		radar *Radar

		resourceConfig atc.ResourceConfig
		savedResource  db.SavedResource

		fakeLease *dbfakes.FakeLease

		process ifrit.Process
	)

	BeforeEach(func() {
		epoch = time.Unix(123, 456).UTC()
		fakeTracker = new(rfakes.FakeTracker)
		fakeRadarDB = new(fakes.FakeRadarDB)
		fakeClock = fakeclock.NewFakeClock(epoch)
		interval = 1 * time.Minute

		fakeRadarDB.GetPipelineNameReturns("some-pipeline")
		radar = NewRadar(fakeTracker, interval, fakeRadarDB, fakeClock)

		resourceConfig = atc.ResourceConfig{
			Name:   "some-resource",
			Type:   "git",
			Source: atc.Source{"uri": "http://example.com"},
		}

		fakeRadarDB.ScopedNameStub = func(thing string) string {
			return "pipeline:" + thing
		}

		fakeRadarDB.GetConfigReturns(atc.Config{
			Resources: atc.ResourceConfigs{
				resourceConfig,
			},
		}, 1, true, nil)

		savedResource = db.SavedResource{
			ID: 39,
			Resource: db.Resource{
				Name: "some-resource",
			},
			Paused: false,
		}

		fakeLease = &dbfakes.FakeLease{}

		fakeRadarDB.GetResourceReturns(savedResource, nil)
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
				times <- fakeClock.Now()
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

		Context("when the lease cannot be acquired", func() {
			BeforeEach(func() {
				fakeRadarDB.LeaseResourceCheckingReturns(nil, false, nil)
			})

			It("does not check", func() {
				Consistently(times).ShouldNot(Receive())
			})
		})

		Context("when the lease can be acquired", func() {
			BeforeEach(func() {
				fakeRadarDB.LeaseResourceCheckingReturns(fakeLease, true, nil)
			})

			It("checks immediately and then on a specified interval", func() {
				Expect(<-times).To(Equal(epoch))

				fakeClock.WaitForWatcherAndIncrement(interval)
				Expect(<-times).To(Equal(epoch.Add(interval)))
			})

			It("constructs the resource of the correct type", func() {
				<-times

				_, metadata, session, typ, tags := fakeTracker.InitArgsForCall(0)
				Expect(metadata).To(Equal(resource.EmptyMetadata{}))
				Expect(session).To(Equal(resource.Session{
					ID: worker.Identifier{
						ResourceID: 39,
						Stage:      db.ContainerStageRun,
					},
					Metadata: worker.Metadata{
						Type:         db.ContainerTypeCheck,
						CheckType:    "git",
						CheckSource:  atc.Source{"uri": "http://example.com"},
						PipelineName: "some-pipeline",
					},
					Ephemeral: true,
				}))

				Expect(typ).To(Equal(resource.ResourceType("git")))
				Expect(tags).To(BeEmpty()) // This allows the check to run on any worker
			})

			Context("when the resource config has a specified check interval", func() {
				BeforeEach(func() {
					resourceConfig.CheckEvery = "10ms"

					fakeRadarDB.GetConfigReturns(atc.Config{
						Resources: atc.ResourceConfigs{
							resourceConfig,
						},
					}, 1, true, nil)
				})

				It("checks using the specified interval instead of the default", func() {
					Expect(<-times).To(Equal(epoch))

					fakeClock.WaitForWatcherAndIncrement(10 * time.Millisecond)
					Expect(<-times).To(Equal(epoch.Add(10 * time.Millisecond)))
				})

				It("leases for the configured interval", func() {
					<-times

					Expect(fakeRadarDB.LeaseResourceCheckingCallCount()).To(Equal(1))

					resourceName, leaseInterval, immediate := fakeRadarDB.LeaseResourceCheckingArgsForCall(0)
					Expect(resourceName).To(Equal("some-resource"))
					Expect(leaseInterval).To(Equal(10 * time.Millisecond))
					Expect(immediate).To(BeFalse())

					Eventually(fakeLease.BreakCallCount).Should(Equal(1))
				})

				Context("when the interval cannot be parsed", func() {
					BeforeEach(func() {
						resourceConfig.CheckEvery = "bad-value"

						fakeRadarDB.GetConfigReturns(atc.Config{
							Resources: atc.ResourceConfigs{
								resourceConfig,
							},
						}, 1, true, nil)
					})

					It("sets the check error and exits with the error", func() {
						Expect(<-process.Wait()).To(HaveOccurred())
						Expect(fakeRadarDB.SetResourceCheckErrorCallCount()).To(Equal(1))

						resourceName, resourceErr := fakeRadarDB.SetResourceCheckErrorArgsForCall(0)
						Expect(resourceName).To(Equal(savedResource))
						Expect(resourceErr).To(MatchError("time: invalid duration bad-value"))
					})
				})
			})

			It("grabs a periodic resource checking lease before checking, breaks lease after done", func() {
				<-times

				Expect(fakeRadarDB.LeaseResourceCheckingCallCount()).To(Equal(1))

				resourceName, leaseInterval, immediate := fakeRadarDB.LeaseResourceCheckingArgsForCall(0)
				Expect(resourceName).To(Equal("some-resource"))
				Expect(leaseInterval).To(Equal(interval))
				Expect(immediate).To(BeFalse())

				Eventually(fakeLease.BreakCallCount).Should(Equal(1))
			})

			It("releases after checking", func() {
				<-times
				Eventually(fakeResource.ReleaseCallCount).Should(Equal(1))
			})

			Context("when there is no current version", func() {
				It("checks from nil", func() {
					<-times

					_, version := fakeResource.CheckArgsForCall(0)
					Expect(version).To(BeNil())
				})
			})

			Context("when there is a current version", func() {
				BeforeEach(func() {
					fakeRadarDB.GetLatestVersionedResourceReturns(
						db.SavedVersionedResource{
							ID: 1,
							VersionedResource: db.VersionedResource{
								Version: db.Version{
									"version": "1",
								},
							},
						}, true, nil)
				})

				It("checks from it", func() {
					<-times

					_, version := fakeResource.CheckArgsForCall(0)
					Expect(version).To(Equal(atc.Version{"version": "1"}))

					fakeRadarDB.GetLatestVersionedResourceReturns(db.SavedVersionedResource{
						ID:                2,
						VersionedResource: db.VersionedResource{Version: db.Version{"version": "2"}},
					}, true, nil)

					fakeClock.WaitForWatcherAndIncrement(interval)
					<-times

					_, version = fakeResource.CheckArgsForCall(1)
					Expect(version).To(Equal(atc.Version{"version": "2"}))
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

						Expect(source).To(Equal(resourceConfig.Source))

						checkedFrom <- from
						result := checkResults[check]
						check++

						return result, nil
					}
				})

				It("saves them all, in order", func() {
					Eventually(fakeRadarDB.SaveResourceVersionsCallCount).Should(Equal(1))

					resourceConfig, versions := fakeRadarDB.SaveResourceVersionsArgsForCall(0)
					Expect(resourceConfig).To(Equal(atc.ResourceConfig{
						Name:   "some-resource",
						Type:   "git",
						Source: atc.Source{"uri": "http://example.com"},
					}))

					Expect(versions).To(Equal([]atc.Version{
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
					Expect(<-process.Wait()).To(Equal(disaster))
				})
			})

			Context("when the pipeline is paused", func() {
				BeforeEach(func() {
					fakeRadarDB.IsPausedReturns(true, nil)
				})

				It("does not check", func() {
					Consistently(times).ShouldNot(Receive())
				})
			})

			Context("when the resource is paused", func() {
				BeforeEach(func() {
					fakeRadarDB.GetResourceReturns(db.SavedResource{
						Resource: db.Resource{
							Name: "some-resource",
						},
						Paused: true,
					}, nil)
				})

				It("does not check", func() {
					Consistently(times).ShouldNot(Receive())
				})
			})

			Context("when checking if the resource is paused fails", func() {
				disaster := errors.New("disaster")

				BeforeEach(func() {
					fakeRadarDB.IsPausedReturns(false, disaster)
				})

				It("exits the process", func() {
					Expect(<-process.Wait()).To(Equal(disaster))
				})
			})

			Context("when checking if the resource is paused fails", func() {
				disaster := errors.New("disaster")

				BeforeEach(func() {
					fakeRadarDB.GetResourceReturns(db.SavedResource{}, disaster)
				})

				It("exits the process", func() {
					Expect(<-process.Wait()).To(Equal(disaster))
				})
			})

			Context("when the config changes", func() {
				var configsToReturn chan<- atc.Config
				var newConfig atc.Config

				BeforeEach(func() {
					configs := make(chan atc.Config, 2)
					configs <- atc.Config{
						Resources: atc.ResourceConfigs{resourceConfig},
					}

					configsToReturn = configs

					fakeRadarDB.GetConfigStub = func() (atc.Config, db.ConfigVersion, bool, error) {
						select {
						case c := <-configs:
							return c, 1, true, nil
						default:
							return newConfig, 2, true, nil
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
						<-times

						source, _ := fakeResource.CheckArgsForCall(0)
						Expect(source).To(Equal(resourceConfig.Source))

						fakeClock.WaitForWatcherAndIncrement(interval)
						<-times

						source, _ = fakeResource.CheckArgsForCall(1)
						Expect(source).To(Equal(atc.Source{"uri": "http://example.com/updated-uri"}))
					})
				})

				Context("with a new interval", func() {
					var (
						newInterval time.Duration
						newResource atc.ResourceConfig
					)

					BeforeEach(func() {
						newInterval = 20 * time.Millisecond
						newResource = resourceConfig
						newResource.CheckEvery = newInterval.String()

						newConfig = atc.Config{
							Resources: atc.ResourceConfigs{newResource},
						}
					})

					It("checks on the new interval", func() {
						<-times // ignore immediate first check

						fakeClock.WaitForWatcherAndIncrement(interval)
						Expect(<-times).To(Equal(epoch.Add(interval)))

						fakeClock.WaitForWatcherAndIncrement(newInterval)
						Expect(<-times).To(Equal(epoch.Add(interval + newInterval)))

						source, _ := fakeResource.CheckArgsForCall(0)
						Expect(source).To(Equal(newResource.Source))
					})

					Context("when the interval cannot be parsed", func() {
						BeforeEach(func() {
							newResource.CheckEvery = "bad-value"

							newConfig = atc.Config{
								Resources: atc.ResourceConfigs{newResource},
							}
						})

						It("sets the check error and exits with the error", func() {
							<-times

							fakeClock.WaitForWatcherAndIncrement(interval)

							Expect(<-process.Wait()).To(HaveOccurred())
							Expect(fakeRadarDB.SetResourceCheckErrorCallCount()).To(Equal(2))

							resourceName, resourceErr := fakeRadarDB.SetResourceCheckErrorArgsForCall(0)
							Expect(resourceName).To(Equal(savedResource))
							Expect(resourceErr).ToNot(HaveOccurred())

							resourceName, resourceErr = fakeRadarDB.SetResourceCheckErrorArgsForCall(1)
							Expect(resourceName).To(Equal(savedResource))
							Expect(resourceErr).To(MatchError("time: invalid duration bad-value"))
						})
					})

					Context("when the interval is removed", func() {
						BeforeEach(func() {
							configsToReturn <- newConfig

							newResource.CheckEvery = ""

							newConfig = atc.Config{
								Resources: atc.ResourceConfigs{newResource},
							}
						})

						It("goes back to the default interval", func() {
							Expect(<-times).To(Equal(epoch)) // ignore immediate first check

							fakeClock.WaitForWatcherAndIncrement(interval)
							Expect(<-times).To(Equal(epoch.Add(interval)))

							fakeClock.WaitForWatcherAndIncrement(newInterval)
							Expect(<-times).To(Equal(epoch.Add(interval + newInterval)))

							fakeClock.WaitForWatcherAndIncrement(newInterval)
							fakeClock.Increment(interval - newInterval)
							Expect(<-times).To(Equal(epoch.Add(interval + newInterval + interval)))
						})
					})
				})

				Context("with the resource removed", func() {
					BeforeEach(func() {
						newConfig = atc.Config{
							Resources: atc.ResourceConfigs{},
						}
					})

					It("exits with the correct error", func() {
						<-times

						fakeClock.WaitForWatcherAndIncrement(interval)

						Expect(<-process.Wait()).To(Equal(ResourceNotConfiguredError{"some-resource"}))
					})
				})
			})

			Context("when checking takes a while", func() {
				BeforeEach(func() {
					fakeResource.CheckStub = func(atc.Source, atc.Version) ([]atc.Version, error) {
						times <- fakeClock.Now()
						fakeClock.Increment(interval / 2)
						return nil, nil
					}
				})

				It("does not count it towards the interval", func() {
					Expect(<-times).To(Equal(epoch))

					fakeClock.WaitForWatcherAndIncrement(interval / 2)
					fakeClock.Increment(interval / 2)
					Expect(<-times).To(Equal(epoch.Add(interval + (interval / 2))))
				})
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

		Context("if the lease can be acquired", func() {
			BeforeEach(func() {
				fakeRadarDB.LeaseResourceCheckingReturns(fakeLease, true, nil)
			})

			It("succeeds", func() {
				Expect(scanErr).NotTo(HaveOccurred())
			})

			It("constructs the resource of the correct type", func() {
				_, metadata, session, typ, tags := fakeTracker.InitArgsForCall(0)
				Expect(metadata).To(Equal(resource.EmptyMetadata{}))
				Expect(session).To(Equal(resource.Session{
					ID: worker.Identifier{
						ResourceID: 39,
						Stage:      db.ContainerStageRun,
					},
					Metadata: worker.Metadata{
						Type:         db.ContainerTypeCheck,
						CheckType:    "git",
						CheckSource:  atc.Source{"uri": "http://example.com"},
						PipelineName: "some-pipeline",
					},
					Ephemeral: true,
				}))

				Expect(typ).To(Equal(resource.ResourceType("git")))
				Expect(tags).To(BeEmpty()) // This allows the check to run on any worker
			})

			It("grabs an immediate resource checking lease before checking, breaks lease after done", func() {
				Expect(fakeRadarDB.LeaseResourceCheckingCallCount()).To(Equal(1))

				resourceName, leaseInterval, immediate := fakeRadarDB.LeaseResourceCheckingArgsForCall(0)
				Expect(resourceName).To(Equal("some-resource"))
				Expect(leaseInterval).To(Equal(interval))
				Expect(immediate).To(BeTrue())

				Expect(fakeLease.BreakCallCount()).To(Equal(1))
			})

			Context("when the resource config has a specified check interval", func() {
				BeforeEach(func() {
					resourceConfig.CheckEvery = "10ms"

					fakeRadarDB.GetConfigReturns(atc.Config{
						Resources: atc.ResourceConfigs{
							resourceConfig,
						},
					}, 1, true, nil)
				})

				It("leases for the configured interval", func() {
					Expect(fakeRadarDB.LeaseResourceCheckingCallCount()).To(Equal(1))

					resourceName, leaseInterval, immediate := fakeRadarDB.LeaseResourceCheckingArgsForCall(0)
					Expect(resourceName).To(Equal("some-resource"))
					Expect(leaseInterval).To(Equal(10 * time.Millisecond))
					Expect(immediate).To(BeTrue())

					Eventually(fakeLease.BreakCallCount).Should(Equal(1))
				})

				Context("when the interval cannot be parsed", func() {
					BeforeEach(func() {
						resourceConfig.CheckEvery = "bad-value"

						fakeRadarDB.GetConfigReturns(atc.Config{
							Resources: atc.ResourceConfigs{
								resourceConfig,
							},
						}, 1, true, nil)
					})

					It("sets the check error and returns the error", func() {
						Expect(scanErr).To(HaveOccurred())
						Expect(fakeRadarDB.SetResourceCheckErrorCallCount()).To(Equal(1))

						resourceName, resourceErr := fakeRadarDB.SetResourceCheckErrorArgsForCall(0)
						Expect(resourceName).To(Equal(savedResource))
						Expect(resourceErr).To(MatchError("time: invalid duration bad-value"))
					})
				})
			})

			Context("when the lease is not immediately available", func() {
				BeforeEach(func() {
					results := make(chan bool, 4)
					results <- false
					results <- false
					results <- true
					results <- true
					close(results)

					fakeRadarDB.LeaseResourceCheckingStub = func(resourceName string, interval time.Duration, immediate bool) (db.Lease, bool, error) {
						if <-results {
							return fakeLease, true, nil
						} else {
							// allow the sleep to continue
							go fakeClock.WaitForWatcherAndIncrement(time.Second)
							return nil, false, nil
						}
					}
				})

				It("retries every second until it is", func() {
					Expect(fakeRadarDB.LeaseResourceCheckingCallCount()).To(Equal(3))

					resourceName, leaseInterval, immediate := fakeRadarDB.LeaseResourceCheckingArgsForCall(0)
					Expect(resourceName).To(Equal("some-resource"))
					Expect(leaseInterval).To(Equal(interval))
					Expect(immediate).To(BeTrue())

					resourceName, leaseInterval, immediate = fakeRadarDB.LeaseResourceCheckingArgsForCall(1)
					Expect(resourceName).To(Equal("some-resource"))
					Expect(leaseInterval).To(Equal(interval))
					Expect(immediate).To(BeTrue())

					resourceName, leaseInterval, immediate = fakeRadarDB.LeaseResourceCheckingArgsForCall(2)
					Expect(resourceName).To(Equal("some-resource"))
					Expect(leaseInterval).To(Equal(interval))
					Expect(immediate).To(BeTrue())

					Expect(fakeLease.BreakCallCount()).To(Equal(1))
				})
			})

			It("releases the resource", func() {
				Expect(fakeResource.ReleaseCallCount()).To(Equal(1))
			})

			It("clears the resource's check error", func() {
				Expect(fakeRadarDB.SetResourceCheckErrorCallCount()).To(Equal(1))

				savedResourceArg, err := fakeRadarDB.SetResourceCheckErrorArgsForCall(0)
				Expect(savedResourceArg).To(Equal(savedResource))
				Expect(err).To(BeNil())
			})

			Context("when there is no current version", func() {
				BeforeEach(func() {
					fakeRadarDB.GetLatestVersionedResourceReturns(db.SavedVersionedResource{}, false, nil)
				})

				It("checks from nil", func() {
					_, version := fakeResource.CheckArgsForCall(0)
					Expect(version).To(BeNil())
				})
			})

			Context("when getting the current version fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeRadarDB.GetLatestVersionedResourceReturns(db.SavedVersionedResource{}, false, disaster)
				})

				It("returns the error", func() {
					Expect(scanErr).To(Equal(disaster))
				})

				It("does not check", func() {
					Expect(fakeResource.CheckCallCount()).To(Equal(0))
				})
			})

			Context("when there is a current version", func() {
				BeforeEach(func() {
					fakeRadarDB.GetLatestVersionedResourceReturns(
						db.SavedVersionedResource{
							ID: 1,
							VersionedResource: db.VersionedResource{
								Version: db.Version{
									"version": "1",
								},
							},
						}, true, nil)
				})

				It("checks from it", func() {
					_, version := fakeResource.CheckArgsForCall(0)
					Expect(version).To(Equal(atc.Version{"version": "1"}))
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

						Expect(source).To(Equal(resourceConfig.Source))

						checkedFrom <- from
						result := checkResults[check]
						check++

						return result, nil
					}
				})

				It("saves them all, in order", func() {
					Expect(fakeRadarDB.SaveResourceVersionsCallCount()).To(Equal(1))

					resourceConfig, versions := fakeRadarDB.SaveResourceVersionsArgsForCall(0)
					Expect(resourceConfig).To(Equal(atc.ResourceConfig{
						Name:   "some-resource",
						Type:   "git",
						Source: atc.Source{"uri": "http://example.com"},
					}))

					Expect(versions).To(Equal([]atc.Version{
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
					Expect(scanErr).To(Equal(disaster))
				})

				It("sets the resource's check error", func() {
					Expect(fakeRadarDB.SetResourceCheckErrorCallCount()).To(Equal(1))

					savedResourceArg, err := fakeRadarDB.SetResourceCheckErrorArgsForCall(0)
					Expect(savedResourceArg).To(Equal(savedResource))
					Expect(err).To(Equal(disaster))
				})
			})
		})
	})
})
