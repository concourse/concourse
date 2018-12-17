package radar_test

import (
	"context"
	"errors"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/concourse/atc/radar"
	"github.com/concourse/concourse/atc/radar/radarfakes"
	"github.com/concourse/concourse/atc/worker"

	. "github.com/concourse/concourse/atc/radar"
	"github.com/concourse/concourse/atc/resource"
	rfakes "github.com/concourse/concourse/atc/resource/resourcefakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceScanner", func() {
	var (
		epoch time.Time

		fakeResourceFactory       *rfakes.FakeResourceFactory
		fakeResourceConfigFactory *dbfakes.FakeResourceConfigFactory
		fakeDBPipeline            *dbfakes.FakePipeline
		fakeClock                 *fakeclock.FakeClock
		interval                  time.Duration
		variables                 creds.Variables

		fakeResourceType      *dbfakes.FakeResourceType
		versionedResourceType atc.VersionedResourceType

		scanner                 Scanner
		fakeResourceTypeScanner *radarfakes.FakeScanner

		resourceConfig     atc.ResourceConfig
		fakeDBResource     *dbfakes.FakeResource
		fakeResourceConfig *dbfakes.FakeResourceConfig

		fakeLock *lockfakes.FakeLock
		teamID   = 123
	)

	BeforeEach(func() {
		epoch = time.Unix(123, 456).UTC()
		fakeLock = &lockfakes.FakeLock{}
		interval = 1 * time.Minute
		GlobalResourceCheckTimeout = 1 * time.Hour
		variables = template.StaticVariables{
			"source-params": "some-secret-sauce",
		}

		resourceConfig = atc.ResourceConfig{
			Name:   "some-resource",
			Type:   "git",
			Source: atc.Source{"uri": "some-secret-sauce"},
			Tags:   atc.Tags{"some-tag"},
		}

		versionedResourceType = atc.VersionedResourceType{
			ResourceType: atc.ResourceType{
				Name:   "some-custom-resource",
				Type:   "registry-image",
				Source: atc.Source{"custom": "((source-params))"},
			},
			Version: atc.Version{"custom": "version"},
		}

		fakeResourceFactory = new(rfakes.FakeResourceFactory)
		fakeResourceConfigFactory = new(dbfakes.FakeResourceConfigFactory)
		fakeResourceType = new(dbfakes.FakeResourceType)
		fakeDBResource = new(dbfakes.FakeResource)
		fakeDBPipeline = new(dbfakes.FakePipeline)
		fakeResourceConfig = new(dbfakes.FakeResourceConfig)
		fakeResourceConfig.IDReturns(123)

		fakeDBPipeline.IDReturns(42)
		fakeDBPipeline.NameReturns("some-pipeline")
		fakeDBPipeline.TeamIDReturns(teamID)
		fakeClock = fakeclock.NewFakeClock(epoch)

		fakeDBPipeline.ReloadReturns(true, nil)
		fakeDBPipeline.ResourceTypesReturns([]db.ResourceType{fakeResourceType}, nil)

		fakeResourceType.IDReturns(1)
		fakeResourceType.NameReturns("some-custom-resource")
		fakeResourceType.TypeReturns("registry-image")
		fakeResourceType.SourceReturns(atc.Source{"custom": "((source-params))"})
		fakeResourceType.VersionReturns(atc.Version{"custom": "version"})

		fakeDBResource.IDReturns(39)
		fakeDBResource.NameReturns("some-resource")
		fakeDBResource.PipelineNameReturns("some-pipeline")
		fakeDBResource.TypeReturns("git")
		fakeDBResource.SourceReturns(atc.Source{"uri": "((source-params))"})
		fakeDBResource.TagsReturns(atc.Tags{"some-tag"})
		fakeDBResource.SetResourceConfigReturns(fakeResourceConfig, nil)

		fakeDBPipeline.ResourceReturns(fakeDBResource, true, nil)

		fakeResourceTypeScanner = new(radarfakes.FakeScanner)

		scanner = NewResourceScanner(
			fakeClock,
			fakeResourceFactory,
			fakeResourceConfigFactory,
			interval,
			fakeDBPipeline,
			"https://www.example.com",
			variables,
			fakeResourceTypeScanner,
		)
	})

	Describe("Run", func() {
		var (
			fakeResource   *rfakes.FakeResource
			actualInterval time.Duration
			runErr         error
		)

		BeforeEach(func() {
			fakeResource = new(rfakes.FakeResource)
			fakeResourceFactory.NewResourceReturns(fakeResource, nil)
		})

		JustBeforeEach(func() {
			actualInterval, runErr = scanner.Run(lagertest.NewTestLogger("test"), "some-resource")
		})

		Context("when the lock cannot be acquired", func() {
			BeforeEach(func() {
				fakeResourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheckReturns(nil, false, nil)
			})

			It("does not check", func() {
				Expect(fakeResource.CheckCallCount()).To(Equal(0))
			})

			It("returns the configured interval", func() {
				Expect(runErr).To(Equal(ErrFailedToAcquireLock))
				Expect(actualInterval).To(Equal(interval))
			})
		})

		Context("when the lock can be acquired", func() {
			BeforeEach(func() {
				fakeResourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheckReturns(fakeLock, true, nil)
			})

			It("checks immediately", func() {
				Expect(fakeResource.CheckCallCount()).To(Equal(1))
			})

			It("constructs the resource of the correct type", func() {
				Expect(fakeDBResource.SetResourceConfigCallCount()).To(Equal(1))
				_, resourceSource, resourceTypes := fakeDBResource.SetResourceConfigArgsForCall(0)
				Expect(resourceSource).To(Equal(atc.Source{"uri": "some-secret-sauce"}))
				Expect(resourceTypes).To(Equal(creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
					versionedResourceType,
				})))

				Expect(fakeDBResource.SetCheckErrorCallCount()).To(Equal(1))
				err := fakeDBResource.SetCheckErrorArgsForCall(0)
				Expect(err).To(BeNil())

				_, _, owner, metadata, containerSpec, workerSpec, resourceTypes, _ := fakeResourceFactory.NewResourceArgsForCall(0)
				Expect(owner).To(Equal(db.NewResourceConfigCheckSessionContainerOwner(fakeResourceConfig, radar.ContainerExpiries)))
				Expect(metadata).To(Equal(db.ContainerMetadata{
					Type: db.ContainerTypeCheck,
				}))
				Expect(containerSpec).To(Equal(worker.ContainerSpec{
					ImageSpec: worker.ImageSpec{
						ResourceType: "git",
					},
					Tags:   atc.Tags{"some-tag"},
					TeamID: 123,
					Env: []string{
						"ATC_EXTERNAL_URL=https://www.example.com",
						"RESOURCE_PIPELINE_NAME=some-pipeline",
						"RESOURCE_NAME=some-resource",
					},
				}))
				Expect(workerSpec).To(Equal(worker.WorkerSpec{
					ResourceType:  "git",
					Tags:          atc.Tags{"some-tag"},
					ResourceTypes: creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{versionedResourceType}),
				}))
				Expect(resourceTypes).To(Equal(creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
					versionedResourceType,
				})))
			})

			Context("when the resource config has a specified check interval", func() {
				BeforeEach(func() {
					fakeDBResource.CheckEveryReturns("10ms")
					fakeDBPipeline.ResourceReturns(fakeDBResource, true, nil)
				})

				It("leases for the configured interval", func() {
					Expect(fakeResourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheckCallCount()).To(Equal(1))

					_, leaseInterval, immediate := fakeResourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheckArgsForCall(0)
					Expect(leaseInterval).To(Equal(10 * time.Millisecond))
					Expect(immediate).To(BeFalse())

					Eventually(fakeLock.ReleaseCallCount).Should(Equal(1))
				})

				It("returns configured interval", func() {
					Expect(actualInterval).To(Equal(10 * time.Millisecond))
				})

				Context("when the interval cannot be parsed", func() {
					BeforeEach(func() {
						fakeDBResource.CheckEveryReturns("bad-value")
						fakeDBPipeline.ResourceReturns(fakeDBResource, true, nil)
					})

					It("sets the check error", func() {
						Expect(fakeDBResource.SetCheckErrorCallCount()).To(Equal(1))

						resourceErr := fakeDBResource.SetCheckErrorArgsForCall(0)
						Expect(resourceErr).To(MatchError("time: invalid duration bad-value"))
					})

					It("returns an error", func() {
						Expect(runErr).To(HaveOccurred())
					})
				})
			})

			It("grabs a periodic resource checking lock before checking, breaks lock after done", func() {
				Expect(fakeResourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheckCallCount()).To(Equal(1))

				_, leaseInterval, immediate := fakeResourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheckArgsForCall(0)
				Expect(leaseInterval).To(Equal(interval))
				Expect(immediate).To(BeFalse())

				Eventually(fakeLock.ReleaseCallCount).Should(Equal(1))
			})

			Context("when the resource uses a custom type", func() {
				BeforeEach(func() {
					fakeDBResource.TypeReturns("some-custom-resource")
				})
				Context("and the custom type has a version", func() {
					It("doesn't scan for new versions of the custom type", func() {
						Expect(fakeResourceTypeScanner.ScanCallCount()).To(Equal(0))
					})
				})

				Context("and the custom type does not have a version", func() {
					BeforeEach(func() {
						fakeResourceType.VersionReturns(nil)
					})

					It("scans for new versions of the custom type", func() {
						Expect(fakeResourceTypeScanner.ScanCallCount()).To(Equal(1))
						_, scannedResourceType := fakeResourceTypeScanner.ScanArgsForCall(0)
						Expect(scannedResourceType).To(Equal("some-custom-resource"))
					})

					Context("when scanning for the custom type fails", func() {
						var typeScanErr = errors.New("type scan failed")
						BeforeEach(func() {
							fakeResourceTypeScanner.ScanReturns(typeScanErr)
						})
						It("returns the error from scanning for the type", func() {
							Expect(runErr).To(Equal(typeScanErr))
						})
					})

					Context("when scanning for the custom type succeeds", func() {
						BeforeEach(func() {
							fakeResourceTypeScanner.ScanReturns(nil)
						})
						It("reloads the resource types", func() {
							Expect(fakeDBPipeline.ResourceTypesCallCount()).To(Equal(2))
						})
					})
				})
			})

			Context("when there is no current version", func() {
				It("checks from nil", func() {
					_, _, version := fakeResource.CheckArgsForCall(0)
					Expect(version).To(BeNil())
				})
			})

			Context("when there is a current version", func() {
				BeforeEach(func() {
					fakeResourceConfigVersion := new(dbfakes.FakeResourceConfigVersion)
					fakeResourceConfigVersion.IDReturns(1)
					fakeResourceConfigVersion.VersionReturns(db.Version{"version": "1"})

					fakeResourceConfig.LatestVersionReturns(fakeResourceConfigVersion, true, nil)
				})

				It("checks from it", func() {
					_, _, version := fakeResource.CheckArgsForCall(0)
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
					fakeResource.CheckStub = func(ctx context.Context, source atc.Source, from atc.Version) ([]atc.Version, error) {
						defer GinkgoRecover()

						Expect(source).To(Equal(resourceConfig.Source))

						checkedFrom <- from
						result := checkResults[check]
						check++

						return result, nil
					}
				})

				It("saves them all, in order", func() {
					Eventually(fakeResourceConfig.SaveVersionsCallCount).Should(Equal(1))

					versions := fakeResourceConfig.SaveVersionsArgsForCall(0)
					Expect(versions).To(Equal([]atc.Version{
						{"version": "1"},
						{"version": "2"},
						{"version": "3"},
					}))
				})

				Context("when saving versions fails", func() {
					BeforeEach(func() {
						fakeResourceConfig.SaveVersionsReturns(errors.New("failed"))
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
					fakeDBPipeline.CheckPausedReturns(true, nil)
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

			Context("when checking if the pipeline is paused fails", func() {
				disaster := errors.New("disaster")

				BeforeEach(func() {
					fakeDBPipeline.CheckPausedReturns(false, disaster)
				})

				It("returns an error", func() {
					Expect(runErr).To(HaveOccurred())
					Expect(runErr).To(Equal(disaster))
				})
			})

			Context("when finding the resource fails", func() {
				disaster := errors.New("disaster")

				BeforeEach(func() {
					fakeDBPipeline.ResourceReturns(nil, false, disaster)
				})

				It("returns an error", func() {
					Expect(runErr).To(HaveOccurred())
					Expect(runErr).To(Equal(disaster))
				})
			})

			Context("when the resource is not in the database", func() {
				BeforeEach(func() {
					fakeDBPipeline.ResourceReturns(nil, false, nil)
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
			fakeResourceFactory.NewResourceReturns(fakeResource, nil)
		})

		JustBeforeEach(func() {
			scanErr = scanner.Scan(lagertest.NewTestLogger("test"), "some-resource")
		})

		Context("if the lock can be acquired", func() {
			BeforeEach(func() {
				fakeResourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheckReturns(fakeLock, true, nil)
			})

			Context("Parent resource has no version and attempt to Scan fails", func() {
				BeforeEach(func() {
					var fakeGitResourceType *dbfakes.FakeResourceType
					fakeGitResourceType = new(dbfakes.FakeResourceType)

					fakeDBPipeline.ResourceTypesReturns([]db.ResourceType{fakeGitResourceType}, nil)

					fakeGitResourceType.IDReturns(5)
					fakeGitResourceType.NameReturns("git")
					fakeGitResourceType.TypeReturns("registry-image")
					fakeGitResourceType.SourceReturns(atc.Source{"custom": "((source-params))"})
					fakeGitResourceType.VersionReturns(nil)

					fakeResourceTypeScanner.ScanReturns(errors.New("some-resource-type-error"))
				})

				It("fails and returns error", func() {
					Expect(fakeResourceTypeScanner.ScanCallCount()).To(Equal(1))
					_, parentTypeName := fakeResourceTypeScanner.ScanArgsForCall(0)
					Expect(parentTypeName).To(Equal("git"))

					Expect(scanErr).To(HaveOccurred())
					Expect(scanErr.Error()).To(Equal("some-resource-type-error"))
				})

				It("saves the error to check_error on resource row in db", func() {
					Expect(fakeDBResource.SetCheckErrorCallCount()).To(Equal(1))

					err := fakeDBResource.SetCheckErrorArgsForCall(0)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("some-resource-type-error"))
				})
			})

			It("succeeds", func() {
				Expect(scanErr).NotTo(HaveOccurred())
			})

			It("constructs the resource of the correct type", func() {
				Expect(fakeDBResource.SetResourceConfigCallCount()).To(Equal(1))
				_, resourceSource, resourceTypes := fakeDBResource.SetResourceConfigArgsForCall(0)
				Expect(resourceSource).To(Equal(atc.Source{"uri": "some-secret-sauce"}))
				Expect(resourceTypes).To(Equal(creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
					versionedResourceType,
				})))

				Expect(fakeDBResource.SetCheckErrorCallCount()).To(Equal(1))
				err := fakeDBResource.SetCheckErrorArgsForCall(0)
				Expect(err).To(BeNil())

				_, _, owner, metadata, containerSpec, workerSpec, resourceTypes, _ := fakeResourceFactory.NewResourceArgsForCall(0)
				Expect(owner).To(Equal(db.NewResourceConfigCheckSessionContainerOwner(fakeResourceConfig, radar.ContainerExpiries)))
				Expect(metadata).To(Equal(db.ContainerMetadata{
					Type: db.ContainerTypeCheck,
				}))
				Expect(containerSpec).To(Equal(worker.ContainerSpec{
					ImageSpec: worker.ImageSpec{
						ResourceType: "git",
					},
					Tags:   atc.Tags{"some-tag"},
					TeamID: 123,
					Env: []string{
						"ATC_EXTERNAL_URL=https://www.example.com",
						"RESOURCE_PIPELINE_NAME=some-pipeline",
						"RESOURCE_NAME=some-resource",
					},
				}))
				Expect(workerSpec).To(Equal(worker.WorkerSpec{
					ResourceType:  "git",
					Tags:          atc.Tags{"some-tag"},
					ResourceTypes: creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{versionedResourceType}),
				}))
				Expect(resourceTypes).To(Equal(creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
					versionedResourceType,
				})))
			})

			It("grabs an immediate resource checking lock before checking, breaks lock after done", func() {
				Expect(fakeResourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheckCallCount()).To(Equal(1))

				_, leaseInterval, immediate := fakeResourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheckArgsForCall(0)
				Expect(leaseInterval).To(Equal(interval))
				Expect(immediate).To(BeTrue())

				Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
			})

			Context("when setting the resource config on the resource fails", func() {
				BeforeEach(func() {
					fakeDBResource.SetResourceConfigReturns(nil, errors.New("catastrophe"))
				})

				It("sets the check error and returns the error", func() {
					Expect(scanErr).To(HaveOccurred())
					Expect(fakeDBResource.SetCheckErrorCallCount()).To(Equal(1))

					resourceErr := fakeDBResource.SetCheckErrorArgsForCall(0)
					Expect(resourceErr).To(MatchError("catastrophe"))
				})
			})

			Context("when creating the resource checker fails", func() {
				BeforeEach(func() {
					fakeResourceFactory.NewResourceReturns(nil, errors.New("catastrophe"))
				})

				It("sets the check error and returns the error", func() {
					Expect(scanErr).To(HaveOccurred())
					Expect(fakeResourceConfig.SetCheckErrorCallCount()).To(Equal(1))

					resourceErr := fakeResourceConfig.SetCheckErrorArgsForCall(0)
					Expect(resourceErr).To(MatchError("catastrophe"))
				})
			})

			Context("when creating the resource checker fails with no global workers", func() {
				BeforeEach(func() {
					fakeResourceFactory.NewResourceReturns(nil, worker.ErrNoGlobalWorkers)
				})

				It("sets the check error and returns no workers error", func() {
					Expect(scanErr).To(HaveOccurred())
					Expect(fakeResourceConfig.SetCheckErrorCallCount()).To(Equal(1))

					resourceErr := fakeResourceConfig.SetCheckErrorArgsForCall(0)
					Expect(resourceErr).To(Equal(atc.ErrNoWorkers))
				})
			})

			Context("when the resource config has a specified check interval", func() {
				BeforeEach(func() {
					fakeDBResource.CheckEveryReturns("10ms")
					fakeDBPipeline.ResourceReturns(fakeDBResource, true, nil)
				})

				It("leases for the configured interval", func() {
					Expect(fakeResourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheckCallCount()).To(Equal(1))

					_, leaseInterval, immediate := fakeResourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheckArgsForCall(0)
					Expect(leaseInterval).To(Equal(10 * time.Millisecond))
					Expect(immediate).To(BeTrue())

					Eventually(fakeLock.ReleaseCallCount).Should(Equal(1))
				})

				Context("when the interval cannot be parsed", func() {
					BeforeEach(func() {
						fakeDBResource.CheckEveryReturns("bad-value")
						fakeDBPipeline.ResourceReturns(fakeDBResource, true, nil)
					})

					It("sets the check error and returns the error", func() {
						Expect(scanErr).To(HaveOccurred())
						Expect(fakeDBResource.SetCheckErrorCallCount()).To(Equal(1))

						resourceErr := fakeDBResource.SetCheckErrorArgsForCall(0)
						Expect(resourceErr).To(MatchError("time: invalid duration bad-value"))
					})
				})
			})

			Context("when the resource has a specified timeout", func() {
				BeforeEach(func() {
					fakeDBResource.CheckTimeoutReturns("10s")
					fakeDBPipeline.ResourceReturns(fakeDBResource, true, nil)
				})

				It("times out after the specified timeout", func() {
					now := time.Now()
					ctx, _, _ := fakeResource.CheckArgsForCall(0)
					deadline, _ := ctx.Deadline()
					Expect(deadline).Should(BeTemporally("~", now.Add(10*time.Second), time.Second))
				})

				Context("when the timeout cannot be parsed", func() {
					BeforeEach(func() {
						fakeDBResource.CheckTimeoutReturns("bad-value")
						fakeDBPipeline.ResourceReturns(fakeDBResource, true, nil)
					})

					It("fails to parse the timeout and returns the error", func() {
						Expect(scanErr).To(HaveOccurred())
						Expect(fakeDBResource.SetCheckErrorCallCount()).To(Equal(1))

						resourceErr := fakeDBResource.SetCheckErrorArgsForCall(0)
						Expect(resourceErr).To(MatchError("time: invalid duration bad-value"))
					})
				})
			})

			Context("when the resource has a pinned version", func() {
				BeforeEach(func() {
					fakeDBResource.CurrentPinnedVersionReturns(atc.Version{"version": "1"})
				})

				It("tries to find the version in the database", func() {
					Expect(fakeResourceConfig.FindVersionCallCount()).To(Equal(1))
					Expect(fakeResourceConfig.FindVersionArgsForCall(0)).To(Equal(atc.Version{"version": "1"}))
				})

				Context("when finding the version succeeds", func() {
					BeforeEach(func() {
						fakeResourceConfigVersion := new(dbfakes.FakeResourceConfigVersion)
						fakeResourceConfigVersion.IDReturns(1)
						fakeResourceConfigVersion.VersionReturns(db.Version{"version": "1"})
						fakeResourceConfig.FindVersionReturns(fakeResourceConfigVersion, true, nil)
					})

					It("does not check", func() {
						Expect(fakeResource.CheckCallCount()).To(Equal(0))
					})
				})

				Context("when the version is not found", func() {
					BeforeEach(func() {
						fakeResourceConfig.FindVersionReturns(nil, false, nil)
					})

					It("checks from the pinned version", func() {
						_, _, version := fakeResource.CheckArgsForCall(0)
						Expect(version).To(Equal(atc.Version{"version": "1"}))
					})
				})

				Context("when finding the version fails", func() {
					BeforeEach(func() {
						fakeResourceConfig.FindVersionReturns(nil, false, errors.New("ah"))
					})

					It("sets the check error on the resource config", func() {
						Expect(fakeResourceConfig.SetCheckErrorCallCount()).To(Equal(1))

						err := fakeResourceConfig.SetCheckErrorArgsForCall(0)
						Expect(err).To(Equal(errors.New("ah")))
					})
				})
			})

			Context("when the lock is not immediately available", func() {
				BeforeEach(func() {
					results := make(chan bool, 4)
					results <- false
					results <- false
					results <- true
					results <- true
					close(results)

					fakeResourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheckStub = func(logger lager.Logger, interval time.Duration, immediate bool) (lock.Lock, bool, error) {
						if <-results {
							return fakeLock, true, nil
						} else {
							// allow the sleep to continue
							go fakeClock.WaitForWatcherAndIncrement(time.Second)
							return nil, false, nil
						}
					}
				})

				It("retries every second until it is", func() {
					Expect(fakeResourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheckCallCount()).To(Equal(3))

					_, leaseInterval, immediate := fakeResourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheckArgsForCall(0)
					Expect(leaseInterval).To(Equal(interval))
					Expect(immediate).To(BeTrue())

					_, leaseInterval, immediate = fakeResourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheckArgsForCall(1)
					Expect(leaseInterval).To(Equal(interval))
					Expect(immediate).To(BeTrue())

					_, leaseInterval, immediate = fakeResourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheckArgsForCall(2)
					Expect(leaseInterval).To(Equal(interval))
					Expect(immediate).To(BeTrue())

					Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
				})
			})

			It("clears the resource's check error", func() {
				Expect(fakeResourceConfig.SetCheckErrorCallCount()).To(Equal(1))

				err := fakeResourceConfig.SetCheckErrorArgsForCall(0)
				Expect(err).To(BeNil())
			})

			Context("when there is no current version", func() {
				BeforeEach(func() {
					fakeResourceConfig.LatestVersionReturns(nil, false, nil)
				})

				It("checks from nil", func() {
					_, _, version := fakeResource.CheckArgsForCall(0)
					Expect(version).To(BeNil())
				})
			})

			Context("when getting the current version fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeResourceConfig.LatestVersionReturns(nil, false, disaster)
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
					fakeResourceConfigVersion := new(dbfakes.FakeResourceConfigVersion)
					fakeResourceConfigVersion.IDReturns(1)
					fakeResourceConfigVersion.VersionReturns(db.Version(latestVersion))

					fakeResourceConfig.LatestVersionReturns(fakeResourceConfigVersion, true, nil)
				})

				It("checks from it", func() {
					_, _, version := fakeResource.CheckArgsForCall(0)
					Expect(version).To(Equal(atc.Version{"version": "1"}))
				})

				Context("when the check returns only the latest version", func() {
					BeforeEach(func() {
						fakeResource.CheckReturns([]atc.Version{atc.Version(latestVersion)}, nil)
					})

					It("does not save it", func() {
						Expect(fakeResourceConfig.SaveVersionsCallCount()).To(Equal(0))
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
					fakeResource.CheckStub = func(ctx context.Context, source atc.Source, from atc.Version) ([]atc.Version, error) {
						defer GinkgoRecover()

						Expect(source).To(Equal(resourceConfig.Source))

						checkedFrom <- from
						result := checkResults[check]
						check++

						return result, nil
					}
				})

				It("saves them all, in order", func() {
					Expect(fakeResourceConfig.SaveVersionsCallCount()).To(Equal(1))

					versions := fakeResourceConfig.SaveVersionsArgsForCall(0)
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
					Expect(fakeResourceConfig.SetCheckErrorCallCount()).To(Equal(1))

					err := fakeResourceConfig.SetCheckErrorArgsForCall(0)
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
					Expect(fakeResourceConfig.SetCheckErrorCallCount()).To(Equal(1))

					err := fakeResourceConfig.SetCheckErrorArgsForCall(0)
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
			fakeResourceFactory.NewResourceReturns(fakeResource, nil)
			fromVersion = nil
		})

		JustBeforeEach(func() {
			scanErr = scanner.ScanFromVersion(lagertest.NewTestLogger("test"), "some-resource", fromVersion)
		})

		Context("if the lock can be acquired", func() {
			BeforeEach(func() {
				fakeResourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheckReturns(fakeLock, true, nil)
			})

			Context("when fromVersion is nil", func() {
				It("checks from nil", func() {
					_, _, version := fakeResource.CheckArgsForCall(0)
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
					_, _, version := fakeResource.CheckArgsForCall(0)
					Expect(version).To(Equal(atc.Version{"version": "1"}))
				})

				Context("when the check returns only the latest version", func() {
					BeforeEach(func() {
						fakeResource.CheckReturns([]atc.Version{fromVersion}, nil)
					})

					It("saves it", func() {
						Expect(fakeResourceConfig.SaveVersionsCallCount()).To(Equal(1))
						versions := fakeResourceConfig.SaveVersionsArgsForCall(0)
						Expect(versions).To(Equal([]atc.Version{fromVersion}))
					})
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
					fakeDBPipeline.ResourceReturns(nil, false, nil)
				})

				It("returns an error", func() {
					Expect(scanErr).To(HaveOccurred())
					Expect(scanErr.Error()).To(ContainSubstring("resource 'some-resource' not found"))
				})
			})
		})
	})
})
