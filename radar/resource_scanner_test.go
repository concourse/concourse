package radar_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"

	"github.com/concourse/atc/db/dbfakes"
	. "github.com/concourse/atc/radar"
	"github.com/concourse/atc/radar/radarfakes"
	"github.com/concourse/atc/resource"
	rfakes "github.com/concourse/atc/resource/resourcefakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceScanner", func() {
	var (
		epoch time.Time

		fakeTracker *rfakes.FakeTracker
		fakeRadarDB *radarfakes.FakeRadarDB
		fakeClock   *fakeclock.FakeClock
		interval    time.Duration

		scanner Scanner

		resourceConfig atc.ResourceConfig
		savedResource  db.SavedResource

		fakeLease *dbfakes.FakeLease
		teamID    = 123
	)

	BeforeEach(func() {
		epoch = time.Unix(123, 456).UTC()
		fakeTracker = new(rfakes.FakeTracker)
		fakeRadarDB = new(radarfakes.FakeRadarDB)
		fakeClock = fakeclock.NewFakeClock(epoch)
		interval = 1 * time.Minute

		fakeRadarDB.GetPipelineIDReturns(42)
		scanner = NewResourceScanner(
			fakeClock,
			fakeTracker,
			interval,
			fakeRadarDB,
			"https://www.example.com",
		)

		resourceConfig = atc.ResourceConfig{
			Name:   "some-resource",
			Type:   "git",
			Source: atc.Source{"uri": "http://example.com"},
		}

		fakeRadarDB.ScopedNameStub = func(thing string) string {
			return "pipeline:" + thing
		}
		fakeRadarDB.TeamIDReturns(teamID)
		fakeRadarDB.GetConfigReturns(atc.Config{
			Resources: atc.ResourceConfigs{
				resourceConfig,
			},
			ResourceTypes: atc.ResourceTypes{
				{
					Name:   "some-custom-resource",
					Type:   "docker-image",
					Source: atc.Source{"custom": "source"},
				},
			},
		}, 1, true, nil)

		savedResource = db.SavedResource{
			ID: 39,
			Resource: db.Resource{
				Name: "some-resource",
			},
			PipelineName: "some-pipeline",
			Paused:       false,
		}

		fakeLease = &dbfakes.FakeLease{}

		fakeRadarDB.GetResourceReturns(savedResource, true, nil)
	})

	Describe("Run", func() {
		var (
			fakeResource   *rfakes.FakeResource
			actualInterval time.Duration
			runErr         error
		)

		BeforeEach(func() {
			fakeResource = new(rfakes.FakeResource)
			fakeTracker.InitReturns(fakeResource, nil)
		})

		JustBeforeEach(func() {
			actualInterval, runErr = scanner.Run(lagertest.NewTestLogger("test"), "some-resource")
		})

		Context("when the lease cannot be acquired", func() {
			BeforeEach(func() {
				fakeRadarDB.LeaseResourceCheckingReturns(nil, false, nil)
			})

			It("does not check", func() {
				Expect(fakeResource.CheckCallCount()).To(Equal(0))
			})

			It("returns the configured interval", func() {
				Expect(runErr).To(Equal(ErrFailedToAcquireLease))
				Expect(actualInterval).To(Equal(interval))
			})
		})

		Context("when the lease can be acquired", func() {
			BeforeEach(func() {
				fakeRadarDB.LeaseResourceCheckingReturns(fakeLease, true, nil)
			})

			It("checks immediately", func() {
				Expect(fakeResource.CheckCallCount()).To(Equal(1))
			})

			It("constructs the resource of the correct type", func() {
				_, metadata, session, typ, tags, actualTeamID, customTypes, delegate := fakeTracker.InitArgsForCall(0)
				Expect(metadata).To(Equal(resource.TrackerMetadata{
					ResourceName: "some-resource",
					PipelineName: "some-pipeline",
					ExternalURL:  "https://www.example.com",
				}))

				Expect(session).To(Equal(resource.Session{
					ID: worker.Identifier{
						ResourceID:  39,
						Stage:       db.ContainerStageRun,
						CheckType:   "git",
						CheckSource: atc.Source{"uri": "http://example.com"},
					},
					Metadata: worker.Metadata{
						Type:       db.ContainerTypeCheck,
						PipelineID: 42,
						TeamID:     teamID,
					},
					Ephemeral: true,
				}))
				Expect(customTypes).To(Equal(atc.ResourceTypes{
					{
						Name:   "some-custom-resource",
						Type:   "docker-image",
						Source: atc.Source{"custom": "source"},
					},
				}))
				Expect(delegate).To(Equal(worker.NoopImageFetchingDelegate{}))

				Expect(typ).To(Equal(resource.ResourceType("git")))
				Expect(tags).To(BeEmpty()) // This allows the check to run on any worker
				Expect(actualTeamID).To(Equal(teamID))
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

					_, resourceName, leaseInterval, immediate := fakeRadarDB.LeaseResourceCheckingArgsForCall(0)
					Expect(resourceName).To(Equal("some-resource"))
					Expect(leaseInterval).To(Equal(10 * time.Millisecond))
					Expect(immediate).To(BeFalse())

					Eventually(fakeLease.BreakCallCount).Should(Equal(1))
				})

				It("returns configured interval", func() {
					Expect(actualInterval).To(Equal(10 * time.Millisecond))
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

					It("sets the check error", func() {
						Expect(fakeRadarDB.SetResourceCheckErrorCallCount()).To(Equal(1))

						resourceName, resourceErr := fakeRadarDB.SetResourceCheckErrorArgsForCall(0)
						Expect(resourceName).To(Equal(savedResource))
						Expect(resourceErr).To(MatchError("time: invalid duration bad-value"))
					})

					It("returns an error", func() {
						Expect(runErr).To(HaveOccurred())
					})
				})
			})

			It("grabs a periodic resource checking lease before checking, breaks lease after done", func() {
				Expect(fakeRadarDB.LeaseResourceCheckingCallCount()).To(Equal(1))

				_, resourceName, leaseInterval, immediate := fakeRadarDB.LeaseResourceCheckingArgsForCall(0)
				Expect(resourceName).To(Equal("some-resource"))
				Expect(leaseInterval).To(Equal(interval))
				Expect(immediate).To(BeFalse())

				Eventually(fakeLease.BreakCallCount).Should(Equal(1))
			})

			It("releases after checking", func() {
				Eventually(fakeResource.ReleaseCallCount).Should(Equal(1))
			})

			Context("when there is no current version", func() {
				It("checks from nil", func() {
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

				Context("when saving versions fails", func() {
					BeforeEach(func() {
						fakeRadarDB.SaveResourceVersionsReturns(errors.New("failed"))
					})

					It("does not return an error", func() {
						Expect(runErr).NotTo(HaveOccurred())
					})
				})
			})

			Context("when checking fails internally", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeResource.CheckReturns(nil, disaster)
				})

				It("exits with the failure", func() {
					Expect(runErr).To(HaveOccurred())
					Expect(runErr).To(Equal(disaster))
				})
			})

			Context("when checking fails with ErrResourceScriptFailed", func() {
				scriptFail := resource.ErrResourceScriptFailed{}

				BeforeEach(func() {
					fakeResource.CheckReturns(nil, scriptFail)
				})

				It("returns no error", func() {
					Expect(runErr).NotTo(HaveOccurred())
				})
			})

			Context("when the pipeline is paused", func() {
				BeforeEach(func() {
					fakeRadarDB.IsPausedReturns(true, nil)
				})

				It("does not check", func() {
					Expect(fakeResource.CheckCallCount()).To(BeZero())
				})

				It("returns the default interval", func() {
					Expect(actualInterval).To(Equal(interval))
				})

				It("does not return an error", func() {
					Expect(runErr).NotTo(HaveOccurred())
				})
			})

			Context("when the resource is paused", func() {
				BeforeEach(func() {
					fakeRadarDB.GetResourceReturns(db.SavedResource{
						Resource: db.Resource{
							Name: "some-resource",
						},
						Paused: true,
					}, true, nil)
				})

				It("does not check", func() {
					Expect(fakeResource.CheckCallCount()).To(BeZero())
				})

				It("returns the default interval", func() {
					Expect(actualInterval).To(Equal(interval))
				})

				It("does not return an error", func() {
					Expect(runErr).NotTo(HaveOccurred())
				})
			})

			Context("when checking if the resource is paused fails", func() {
				disaster := errors.New("disaster")

				BeforeEach(func() {
					fakeRadarDB.IsPausedReturns(false, disaster)
				})

				It("returns an error", func() {
					Expect(runErr).To(HaveOccurred())
					Expect(runErr).To(Equal(disaster))
				})
			})

			Context("when checking if the resource is paused fails", func() {
				disaster := errors.New("disaster")

				BeforeEach(func() {
					fakeRadarDB.GetResourceReturns(db.SavedResource{}, false, disaster)
				})

				It("returns an error", func() {
					Expect(runErr).To(HaveOccurred())
					Expect(runErr).To(Equal(disaster))
				})
			})

			Context("when the resource is not in the database", func() {
				BeforeEach(func() {
					fakeRadarDB.GetResourceReturns(db.SavedResource{}, false, nil)
				})

				It("returns an error", func() {
					Expect(runErr).To(HaveOccurred())
					Expect(runErr.Error()).To(ContainSubstring("resource 'some-resource' not found"))
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
			scanErr = scanner.Scan(lagertest.NewTestLogger("test"), "some-resource")
		})

		Context("if the lease can be acquired", func() {
			BeforeEach(func() {
				fakeRadarDB.LeaseResourceCheckingReturns(fakeLease, true, nil)
			})

			It("succeeds", func() {
				Expect(scanErr).NotTo(HaveOccurred())
			})

			It("constructs the resource of the correct type", func() {
				_, metadata, session, typ, tags, actualTeamID, _, _ := fakeTracker.InitArgsForCall(0)
				Expect(metadata).To(Equal(resource.TrackerMetadata{
					ResourceName: "some-resource",
					PipelineName: "some-pipeline",
					ExternalURL:  "https://www.example.com",
				}))

				Expect(session).To(Equal(resource.Session{
					ID: worker.Identifier{
						ResourceID:  39,
						Stage:       db.ContainerStageRun,
						CheckType:   "git",
						CheckSource: atc.Source{"uri": "http://example.com"},
					},
					Metadata: worker.Metadata{
						Type:       db.ContainerTypeCheck,
						PipelineID: 42,
						TeamID:     teamID,
					},
					Ephemeral: true,
				}))

				Expect(typ).To(Equal(resource.ResourceType("git")))
				Expect(tags).To(BeEmpty()) // This allows the check to run on any worker
				Expect(actualTeamID).To(Equal(teamID))
			})

			It("grabs an immediate resource checking lease before checking, breaks lease after done", func() {
				Expect(fakeRadarDB.LeaseResourceCheckingCallCount()).To(Equal(1))

				_, resourceName, leaseInterval, immediate := fakeRadarDB.LeaseResourceCheckingArgsForCall(0)
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

					_, resourceName, leaseInterval, immediate := fakeRadarDB.LeaseResourceCheckingArgsForCall(0)
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

					fakeRadarDB.LeaseResourceCheckingStub = func(logger lager.Logger, resourceName string, interval time.Duration, immediate bool) (db.Lease, bool, error) {
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

					_, resourceName, leaseInterval, immediate := fakeRadarDB.LeaseResourceCheckingArgsForCall(0)
					Expect(resourceName).To(Equal("some-resource"))
					Expect(leaseInterval).To(Equal(interval))
					Expect(immediate).To(BeTrue())

					_, resourceName, leaseInterval, immediate = fakeRadarDB.LeaseResourceCheckingArgsForCall(1)
					Expect(resourceName).To(Equal("some-resource"))
					Expect(leaseInterval).To(Equal(interval))
					Expect(immediate).To(BeTrue())

					_, resourceName, leaseInterval, immediate = fakeRadarDB.LeaseResourceCheckingArgsForCall(2)
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
				var latestVersion db.Version
				BeforeEach(func() {
					latestVersion = db.Version{"version": "1"}
					fakeRadarDB.GetLatestVersionedResourceReturns(
						db.SavedVersionedResource{
							ID: 1,
							VersionedResource: db.VersionedResource{
								Version: latestVersion,
							},
						}, true, nil)
				})

				It("checks from it", func() {
					_, version := fakeResource.CheckArgsForCall(0)
					Expect(version).To(Equal(atc.Version{"version": "1"}))
				})

				Context("when the check returns only the latest version", func() {
					BeforeEach(func() {
						fakeResource.CheckReturns([]atc.Version{atc.Version(latestVersion)}, nil)
					})

					It("does not save it", func() {
						Expect(fakeRadarDB.SaveResourceVersionsCallCount()).To(Equal(0))
					})
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

			Context("when checking fails internally", func() {
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

			Context("when checking fails with ErrResourceScriptFailed", func() {
				scriptFail := resource.ErrResourceScriptFailed{}

				BeforeEach(func() {
					fakeResource.CheckReturns(nil, scriptFail)
				})

				It("returns no error", func() {
					Expect(scanErr).NotTo(HaveOccurred())
				})

				It("sets the resource's check error", func() {
					Expect(fakeRadarDB.SetResourceCheckErrorCallCount()).To(Equal(1))

					savedResourceArg, err := fakeRadarDB.SetResourceCheckErrorArgsForCall(0)
					Expect(savedResourceArg).To(Equal(savedResource))
					Expect(err).To(Equal(scriptFail))
				})
			})
		})
	})

	Describe("ScanFromVersion", func() {
		var (
			fakeResource *rfakes.FakeResource
			fromVersion  atc.Version

			scanErr error
		)

		BeforeEach(func() {
			fakeResource = new(rfakes.FakeResource)
			fakeTracker.InitReturns(fakeResource, nil)

			fromVersion = nil
		})

		JustBeforeEach(func() {
			scanErr = scanner.ScanFromVersion(lagertest.NewTestLogger("test"), "some-resource", fromVersion)
		})

		Context("if the lease can be acquired", func() {
			BeforeEach(func() {
				fakeRadarDB.LeaseResourceCheckingReturns(fakeLease, true, nil)
			})

			Context("when fromVersion is nil", func() {
				It("checks from nil", func() {
					_, version := fakeResource.CheckArgsForCall(0)
					Expect(version).To(BeNil())
				})
			})

			Context("when fromVersion is specified", func() {
				BeforeEach(func() {
					fromVersion = atc.Version{
						"version": "1",
					}
				})

				It("checks from it", func() {
					_, version := fakeResource.CheckArgsForCall(0)
					Expect(version).To(Equal(atc.Version{"version": "1"}))
				})
			})

			Context("when checking fails with ErrResourceScriptFailed", func() {
				scriptFail := resource.ErrResourceScriptFailed{}

				BeforeEach(func() {
					fakeResource.CheckReturns(nil, scriptFail)
				})

				It("returns the error", func() {
					Expect(scanErr).To(Equal(scriptFail))
				})
			})

			Context("when the resource is not in the database", func() {
				BeforeEach(func() {
					fakeRadarDB.GetResourceReturns(db.SavedResource{}, false, nil)
				})

				It("returns an error", func() {
					Expect(scanErr).To(HaveOccurred())
					Expect(scanErr.Error()).To(ContainSubstring("resource 'some-resource' not found"))
				})
			})
		})
	})
})
