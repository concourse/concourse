package radar_test

import (
	"errors"
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"
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
		fakeTracker *rfakes.FakeTracker
		fakeRadarDB *fakes.FakeRadarDB
		interval    time.Duration

		radar *Radar

		resourceConfig atc.ResourceConfig
		savedResource  db.SavedResource

		fakeLease *dbfakes.FakeLease

		process ifrit.Process
	)

	BeforeEach(func() {
		fakeTracker = new(rfakes.FakeTracker)
		fakeRadarDB = new(fakes.FakeRadarDB)
		interval = 100 * time.Millisecond

		fakeRadarDB.GetPipelineNameReturns("some-pipeline-name")
		radar = NewRadar(fakeTracker, interval, fakeRadarDB)

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

		Context("if the lease cannot be acquired", func() {
			BeforeEach(func() {
				fakeRadarDB.LeaseResourceCheckingReturns(nil, false, nil)
			})

			It("grabs a resource checking lease before checking, breaks lease after done", func() {
				Consistently(times, 500*time.Millisecond).ShouldNot(Receive())
			})
		})

		Context("if the lease can be acquired", func() {
			BeforeEach(func() {
				fakeRadarDB.LeaseResourceCheckingReturns(fakeLease, true, nil)
			})

			It("constructs the resource of the correct type", func() {
				Eventually(times).Should(Receive())

				metadata, sessionID, typ, tags, vol := fakeTracker.InitArgsForCall(0)
				Ω(metadata).Should(Equal(resource.EmptyMetadata{}))
				Ω(sessionID).Should(Equal(resource.Session{
					ID: worker.Identifier{
						PipelineName: "some-pipeline-name",

						Name: "some-resource",
						Type: "check",

						CheckType:   "git",
						CheckSource: resourceConfig.Source,
					},
					Ephemeral: true,
				}))
				Ω(typ).Should(Equal(resource.ResourceType("git")))
				Ω(tags).Should(BeEmpty()) // This allows the check to run on any worker
				Ω(vol).Should(BeZero())
			})

			It("checks on a specified interval", func() {
				var time1 time.Time
				var time2 time.Time

				Eventually(times).Should(Receive(&time1))
				Eventually(times).Should(Receive(&time2))

				Ω(time2.Sub(time1)).Should(BeNumerically("~", interval, interval/4))
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

				It("should check using the specified interval instead of the default", func() {
					var time1 time.Time
					var time2 time.Time

					Eventually(times).Should(Receive(&time1))
					Eventually(times).Should(Receive(&time2))

					Ω(time2.Sub(time1)).Should(BeNumerically("~", 10*time.Millisecond, 5*time.Millisecond))
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
						Expect(fakeRadarDB.SetResourceCheckErrorCallCount()).To(Equal(2))

						resourceName, resourceErr := fakeRadarDB.SetResourceCheckErrorArgsForCall(0)
						Expect(resourceName).To(Equal(savedResource))
						Expect(resourceErr).ToNot(HaveOccurred())

						resourceName, resourceErr = fakeRadarDB.SetResourceCheckErrorArgsForCall(1)
						Expect(resourceName).To(Equal(savedResource))
						Expect(resourceErr).To(MatchError("time: invalid duration bad-value"))
					})
				})
			})

			It("grabs a periodic resource checking lease before checking, breaks lease after done", func() {
				Eventually(times).Should(Receive())

				Ω(fakeRadarDB.LeaseResourceCheckingCallCount()).Should(Equal(1))

				resourceName, leaseInterval, immediate := fakeRadarDB.LeaseResourceCheckingArgsForCall(0)
				Ω(resourceName).Should(Equal("some-resource"))
				Ω(leaseInterval).Should(Equal(interval))
				Ω(immediate).Should(BeFalse())

				Ω(fakeLease.BreakCallCount()).Should(Equal(1))
			})

			It("releases after checking", func() {
				Eventually(times).Should(Receive())
				Eventually(fakeResource.ReleaseCallCount).Should(Equal(1))
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
					Eventually(times).Should(Receive())

					_, version := fakeResource.CheckArgsForCall(0)
					Ω(version).Should(Equal(atc.Version{"version": "1"}))

					fakeRadarDB.GetLatestVersionedResourceReturns(db.SavedVersionedResource{
						ID:                2,
						VersionedResource: db.VersionedResource{Version: db.Version{"version": "2"}},
					}, true, nil)

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
					Eventually(fakeRadarDB.SaveResourceVersionsCallCount).Should(Equal(1))

					resourceConfig, versions := fakeRadarDB.SaveResourceVersionsArgsForCall(0)
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

			Context("when the pipeline is paused", func() {
				BeforeEach(func() {
					fakeRadarDB.IsPausedReturns(true, nil)
				})

				It("exits the process", func() {
					Consistently(times, 500*time.Millisecond).ShouldNot(Receive())
				})
			})

			Context("when checking if the resource is paused fails", func() {
				disaster := errors.New("disaster")

				BeforeEach(func() {
					fakeRadarDB.IsPausedReturns(false, disaster)
				})

				It("exits the process", func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))
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

				It("exits the process", func() {
					Consistently(times, 500*time.Millisecond).ShouldNot(Receive())
				})
			})

			Context("when checking if the resource is paused fails", func() {
				disaster := errors.New("disaster")

				BeforeEach(func() {
					fakeRadarDB.GetResourceReturns(db.SavedResource{}, disaster)
				})

				It("exits the process", func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))
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
						Eventually(times).Should(Receive())

						source, _ := fakeResource.CheckArgsForCall(0)
						Ω(source).Should(Equal(resourceConfig.Source))

						Eventually(times).Should(Receive())

						source, _ = fakeResource.CheckArgsForCall(1)
						Ω(source).Should(Equal(atc.Source{"uri": "http://example.com/updated-uri"}))
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
						var time1 time.Time
						var time2 time.Time

						Eventually(times).Should(Receive()) // ignore immediate first check

						Eventually(times).Should(Receive(&time1))

						source, _ := fakeResource.CheckArgsForCall(0)
						Ω(source).Should(Equal(newResource.Source))

						Eventually(times).Should(Receive(&time2))
						Ω(time2.Sub(time1)).Should(BeNumerically("~", newInterval, newInterval/2))
					})

					Context("when the interval cannot be parsed", func() {
						BeforeEach(func() {
							newResource.CheckEvery = "bad-value"

							newConfig = atc.Config{
								Resources: atc.ResourceConfigs{newResource},
							}
						})

						It("sets the check error and exits with the error", func() {
							Expect(<-process.Wait()).To(HaveOccurred())
							Expect(fakeRadarDB.SetResourceCheckErrorCallCount()).To(Equal(3))

							resourceName, resourceErr := fakeRadarDB.SetResourceCheckErrorArgsForCall(0)
							Expect(resourceName).To(Equal(savedResource))
							Expect(resourceErr).ToNot(HaveOccurred())

							resourceName, resourceErr = fakeRadarDB.SetResourceCheckErrorArgsForCall(1)
							Expect(resourceName).To(Equal(savedResource))
							Expect(resourceErr).ToNot(HaveOccurred())

							resourceName, resourceErr = fakeRadarDB.SetResourceCheckErrorArgsForCall(2)
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
							var time0 time.Time
							Eventually(times).Should(Receive(&time0)) // ignore immediate first check

							var time1 time.Time
							Eventually(times).Should(Receive(&time1))
							Ω(time1.Sub(time0)).Should(BeNumerically("~", interval, interval/2))

							var time2 time.Time
							Eventually(times).Should(Receive(&time2))
							Ω(time2.Sub(time1)).Should(BeNumerically("~", newInterval, newInterval/2))

							var time3 time.Time
							Eventually(times).Should(Receive(&time3))
							Ω(time3.Sub(time2)).Should(BeNumerically("~", interval, interval/2))
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
						var resourceRemovedError error
						Eventually(process.Wait()).Should(Receive(&resourceRemovedError))
						Expect(resourceRemovedError.Error()).To(Equal("resource 'some-resource' was not found in config"))
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
				Ω(scanErr).ShouldNot(HaveOccurred())
			})

			It("constructs the resource of the correct type", func() {
				metadata, sessionID, typ, tags, vol := fakeTracker.InitArgsForCall(0)
				Ω(metadata).Should(Equal(resource.EmptyMetadata{}))
				Ω(sessionID).Should(Equal(resource.Session{
					ID: worker.Identifier{
						PipelineName: "some-pipeline-name",
						Name:         "some-resource",
						Type:         "check",

						CheckType:   "git",
						CheckSource: resourceConfig.Source,
					},
					Ephemeral: true,
				}))
				Ω(typ).Should(Equal(resource.ResourceType("git")))
				Ω(tags).Should(BeEmpty()) // This allows the check to run on any worker
				Ω(vol).Should(BeZero())
			})

			It("grabs an immediate resource checking lease before checking, breaks lease after done", func() {
				Ω(fakeRadarDB.LeaseResourceCheckingCallCount()).Should(Equal(1))

				resourceName, leaseInterval, immediate := fakeRadarDB.LeaseResourceCheckingArgsForCall(0)
				Ω(resourceName).Should(Equal("some-resource"))
				Ω(leaseInterval).Should(Equal(interval))
				Ω(immediate).Should(BeTrue())

				Ω(fakeLease.BreakCallCount()).Should(Equal(1))
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
							return nil, false, nil
						}
					}
				})

				It("retries until it is", func() {
					Ω(fakeRadarDB.LeaseResourceCheckingCallCount()).Should(Equal(3))

					resourceName, leaseInterval, immediate := fakeRadarDB.LeaseResourceCheckingArgsForCall(0)
					Ω(resourceName).Should(Equal("some-resource"))
					Ω(leaseInterval).Should(Equal(interval))
					Ω(immediate).Should(BeTrue())

					resourceName, leaseInterval, immediate = fakeRadarDB.LeaseResourceCheckingArgsForCall(1)
					Ω(resourceName).Should(Equal("some-resource"))
					Ω(leaseInterval).Should(Equal(interval))
					Ω(immediate).Should(BeTrue())

					resourceName, leaseInterval, immediate = fakeRadarDB.LeaseResourceCheckingArgsForCall(2)
					Ω(resourceName).Should(Equal("some-resource"))
					Ω(leaseInterval).Should(Equal(interval))
					Ω(immediate).Should(BeTrue())

					Ω(fakeLease.BreakCallCount()).Should(Equal(1))
				})
			})

			It("releases the resource", func() {
				Ω(fakeResource.ReleaseCallCount()).Should(Equal(1))
			})

			It("clears the resource's check error", func() {
				Ω(fakeRadarDB.SetResourceCheckErrorCallCount()).Should(Equal(1))

				savedResourceArg, err := fakeRadarDB.SetResourceCheckErrorArgsForCall(0)
				Ω(savedResourceArg).Should(Equal(savedResource))
				Ω(err).Should(BeNil())
			})

			Context("when there is no current version", func() {
				BeforeEach(func() {
					fakeRadarDB.GetLatestVersionedResourceReturns(db.SavedVersionedResource{}, false, nil)
				})

				It("checks from nil", func() {
					_, version := fakeResource.CheckArgsForCall(0)
					Ω(version).Should(BeNil())
				})
			})

			Context("when getting the current version fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeRadarDB.GetLatestVersionedResourceReturns(db.SavedVersionedResource{}, false, disaster)
				})

				It("returns the error", func() {
					Ω(scanErr).Should(Equal(disaster))
				})

				It("does not check", func() {
					Ω(fakeResource.CheckCallCount()).Should(Equal(0))
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
					Ω(fakeRadarDB.SaveResourceVersionsCallCount()).Should(Equal(1))

					resourceConfig, versions := fakeRadarDB.SaveResourceVersionsArgsForCall(0)
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
					Ω(fakeRadarDB.SetResourceCheckErrorCallCount()).Should(Equal(1))

					savedResourceArg, err := fakeRadarDB.SetResourceCheckErrorArgsForCall(0)
					Ω(savedResourceArg).Should(Equal(savedResource))
					Ω(err).Should(Equal(disaster))
				})
			})
		})
	})
})
