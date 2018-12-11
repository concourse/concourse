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
	. "github.com/concourse/concourse/atc/radar"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/worker"

	rfakes "github.com/concourse/concourse/atc/resource/resourcefakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceTypeScanner", func() {
	var (
		epoch time.Time

		fakeResourceFactory       *rfakes.FakeResourceFactory
		fakeResourceConfigFactory *dbfakes.FakeResourceConfigFactory
		fakeDBPipeline            *dbfakes.FakePipeline
		fakeResourceConfig        *dbfakes.FakeResourceConfig
		fakeClock                 *fakeclock.FakeClock
		interval                  time.Duration
		variables                 creds.Variables

		fakeResourceType      *dbfakes.FakeResourceType
		versionedResourceType atc.VersionedResourceType

		scanner Scanner

		fakeLock *lockfakes.FakeLock
		teamID   = 123
	)

	BeforeEach(func() {
		fakeLock = &lockfakes.FakeLock{}
		interval = 1 * time.Minute
		variables = template.StaticVariables{
			"source-params": "some-secret-sauce",
		}

		versionedResourceType = atc.VersionedResourceType{
			ResourceType: atc.ResourceType{
				Name:   "some-custom-resource",
				Type:   "registry-image",
				Source: atc.Source{"custom": "((source-params))"},
				Tags:   atc.Tags{"some-tag"},
			},
			Version: atc.Version{"custom": "version"},
		}

		fakeClock = fakeclock.NewFakeClock(epoch)

		fakeResourceFactory = new(rfakes.FakeResourceFactory)
		fakeResourceConfigFactory = new(dbfakes.FakeResourceConfigFactory)
		fakeResourceType = new(dbfakes.FakeResourceType)
		fakeDBPipeline = new(dbfakes.FakePipeline)
		fakeResourceConfig = new(dbfakes.FakeResourceConfig)
		fakeResourceConfig.IDReturns(123)
		fakeResourceConfigFactory.FindOrCreateResourceConfigReturns(fakeResourceConfig, nil)

		fakeResourceType.IDReturns(39)
		fakeResourceType.NameReturns("some-custom-resource")
		fakeResourceType.TypeReturns("registry-image")
		fakeResourceType.SourceReturns(atc.Source{"custom": "((source-params))"})
		fakeResourceType.VersionReturns(atc.Version{"custom": "version"})
		fakeResourceType.TagsReturns(atc.Tags{"some-tag"})
		fakeResourceType.SetResourceConfigReturns(nil)

		fakeDBPipeline.IDReturns(42)
		fakeDBPipeline.NameReturns("some-pipeline")
		fakeDBPipeline.TeamIDReturns(teamID)
		fakeDBPipeline.ReloadReturns(true, nil)
		fakeDBPipeline.ResourceTypesReturns([]db.ResourceType{fakeResourceType}, nil)
		fakeDBPipeline.ResourceTypeReturns(fakeResourceType, true, nil)

		scanner = NewResourceTypeScanner(
			fakeClock,
			fakeResourceFactory,
			fakeResourceConfigFactory,
			interval,
			fakeDBPipeline,
			"https://www.example.com",
			variables,
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
			actualInterval, runErr = scanner.Run(lagertest.NewTestLogger("test"), fakeResourceType.Name())
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
				Expect(fakeResourceConfigFactory.FindOrCreateResourceConfigCallCount()).To(Equal(1))
				_, resourceType, resourceSource, resourceTypes := fakeResourceConfigFactory.FindOrCreateResourceConfigArgsForCall(0)
				Expect(resourceType).To(Equal("registry-image"))
				Expect(resourceSource).To(Equal(atc.Source{"custom": "some-secret-sauce"}))
				Expect(resourceTypes).To(Equal(creds.VersionedResourceTypes{}))

				Expect(fakeResourceType.SetResourceConfigCallCount()).To(Equal(1))
				resourceConfigID := fakeResourceType.SetResourceConfigArgsForCall(0)
				Expect(resourceConfigID).To(Equal(123))

				Expect(fakeResourceFactory.NewResourceCallCount()).To(Equal(1))
				_, _, owner, metadata, containerSpec, workerSpec, resourceTypes, _ := fakeResourceFactory.NewResourceArgsForCall(0)
				Expect(owner).To(Equal(db.NewResourceConfigCheckSessionContainerOwner(fakeResourceConfig, ContainerExpiries)))
				Expect(metadata).To(Equal(db.ContainerMetadata{
					Type: db.ContainerTypeCheck,
				}))
				Expect(containerSpec).To(Equal(worker.ContainerSpec{
					ImageSpec: worker.ImageSpec{
						ResourceType: "registry-image",
					},
					Tags:   []string{"some-tag"},
					TeamID: 123,
				}))
				Expect(workerSpec).To(Equal(worker.WorkerSpec{
					ResourceType:  "registry-image",
					Tags:          []string{"some-tag"},
					ResourceTypes: creds.VersionedResourceTypes{},
				}))
				Expect(resourceTypes).To(Equal(creds.VersionedResourceTypes{}))
			})

			Context("when the resource type overrides a base resource type", func() {
				BeforeEach(func() {
					otherResourceType := fakeResourceType

					fakeResourceType = new(dbfakes.FakeResourceType)
					fakeResourceType.IDReturns(40)
					fakeResourceType.NameReturns("registry-image")
					fakeResourceType.TypeReturns("registry-image")
					fakeResourceType.SourceReturns(atc.Source{"custom": "((source-params))"})
					fakeResourceType.VersionReturns(atc.Version{"custom": "image-version"})

					fakeDBPipeline.ResourceTypesReturns([]db.ResourceType{
						fakeResourceType,
						otherResourceType,
					}, nil)
					fakeDBPipeline.ResourceTypeReturns(fakeResourceType, true, nil)
					fakeResourceType.SetResourceConfigReturns(nil)
				})

				It("constructs the resource of the correct type", func() {
					Expect(fakeResourceConfigFactory.FindOrCreateResourceConfigCallCount()).To(Equal(1))
					_, resourceType, resourceSource, resourceTypes := fakeResourceConfigFactory.FindOrCreateResourceConfigArgsForCall(0)
					Expect(resourceType).To(Equal("registry-image"))
					Expect(resourceSource).To(Equal(atc.Source{"custom": "some-secret-sauce"}))
					Expect(resourceTypes).To(Equal(creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
						versionedResourceType,
					})))

					Expect(fakeResourceType.SetCheckErrorCallCount()).To(Equal(1))
					err := fakeResourceType.SetCheckErrorArgsForCall(0)
					Expect(err).To(BeNil())

					Expect(fakeResourceType.SetResourceConfigCallCount()).To(Equal(1))
					resourceConfigID := fakeResourceType.SetResourceConfigArgsForCall(0)
					Expect(resourceConfigID).To(Equal(123))

					Expect(fakeResourceFactory.NewResourceCallCount()).To(Equal(1))
					_, _, owner, metadata, containerSpec, workerSpec, resourceTypes, _ := fakeResourceFactory.NewResourceArgsForCall(0)
					Expect(owner).To(Equal(db.NewResourceConfigCheckSessionContainerOwner(fakeResourceConfig, ContainerExpiries)))
					Expect(metadata).To(Equal(db.ContainerMetadata{
						Type: db.ContainerTypeCheck,
					}))
					Expect(containerSpec).To(Equal(worker.ContainerSpec{
						ImageSpec: worker.ImageSpec{
							ResourceType: "registry-image",
						},
						TeamID: 123,
					}))
					Expect(workerSpec).To(Equal(worker.WorkerSpec{
						ResourceType:  "registry-image",
						ResourceTypes: creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{versionedResourceType}),
					}))
					Expect(resourceTypes).To(Equal(creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
						versionedResourceType,
					})))
				})
			})

			Context("when the resource type config has a specified check interval", func() {
				BeforeEach(func() {
					fakeResourceType.CheckEveryReturns("10ms")
					fakeDBPipeline.ResourceTypeReturns(fakeResourceType, true, nil)
				})

				It("leases for the configured interval", func() {
					Expect(fakeResourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheckCallCount()).To(Equal(1))

					_, leaseInterval, immediate := fakeResourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheckArgsForCall(0)
					Expect(leaseInterval).To(Equal(10 * time.Millisecond))
					Expect(immediate).To(BeFalse())

					Eventually(fakeLock.ReleaseCallCount()).Should(Equal(1))
				})

				It("returns configured interval", func() {
					Expect(actualInterval).To(Equal(10 * time.Millisecond))
				})

				Context("when the interval cannot be parsed", func() {
					BeforeEach(func() {
						fakeResourceType.CheckEveryReturns("bad-value")
						fakeDBPipeline.ResourceTypeReturns(fakeResourceType, true, nil)
					})

					It("sets the check error", func() {
						Expect(fakeResourceType.SetCheckErrorCallCount()).To(Equal(1))

						resourceErr := fakeResourceType.SetCheckErrorArgsForCall(0)
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

				Eventually(fakeLock.ReleaseCallCount()).Should(Equal(1))
			})

			Context("when there is no current version", func() {
				BeforeEach(func() {
					fakeResourceType.VersionReturns(nil)
				})

				It("checks from nil", func() {
					_, _, version := fakeResource.CheckArgsForCall(0)
					Expect(version).To(BeNil())
				})
			})

			Context("when there is a current version", func() {
				BeforeEach(func() {
					fakeResourceConfigVersion := new(dbfakes.FakeResourceConfigVersion)
					fakeResourceConfigVersion.IDReturns(1)
					fakeResourceConfigVersion.VersionReturns(db.Version{"version": "42"})

					fakeResourceConfig.LatestVersionReturns(fakeResourceConfigVersion, true, nil)
				})

				It("checks with it", func() {
					Expect(fakeResource.CheckCallCount()).To(Equal(1))
					_, _, version := fakeResource.CheckArgsForCall(0)
					Expect(version).To(Equal(atc.Version{"version": "42"}))
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

						Expect(source).To(Equal(atc.Source{"custom": "some-secret-sauce"}))

						checkedFrom <- from
						result := checkResults[check]
						check++

						return result, nil
					}
				})

				It("saves all resource type versions", func() {
					Eventually(fakeResourceConfig.SaveVersionsCallCount).Should(Equal(1))

					version := fakeResourceConfig.SaveVersionsArgsForCall(0)
					Expect(version).To(Equal(nextVersions))
				})
			})

			Context("when checking fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeResource.CheckReturns(nil, disaster)
				})

				It("exits with the failure", func() {
					Expect(runErr).To(HaveOccurred())
					Expect(runErr).To(Equal(disaster))
				})

				It("sets the resource's check error", func() {
					Expect(fakeResourceConfig.SetCheckErrorCallCount()).To(Equal(1))

					err := fakeResourceConfig.SetCheckErrorArgsForCall(0)
					Expect(err).To(Equal(disaster))
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
		})
	})

	Describe("Scan", func() {
		var (
			fakeResource *rfakes.FakeResource
			runErr       error
		)

		BeforeEach(func() {
			fakeResource = new(rfakes.FakeResource)
			fakeResourceFactory.NewResourceReturns(fakeResource, nil)
		})

		JustBeforeEach(func() {
			runErr = scanner.Scan(lagertest.NewTestLogger("test"), fakeResourceType.Name())
		})

		Context("when the lock can be acquired", func() {
			BeforeEach(func() {
				fakeResourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheckReturns(fakeLock, true, nil)
			})

			It("checks immediately", func() {
				Expect(fakeResource.CheckCallCount()).To(Equal(1))
			})

			It("constructs the resource of the correct type", func() {
				Expect(fakeResourceConfigFactory.FindOrCreateResourceConfigCallCount()).To(Equal(1))
				_, resourceType, resourceSource, resourceTypes := fakeResourceConfigFactory.FindOrCreateResourceConfigArgsForCall(0)
				Expect(resourceType).To(Equal("registry-image"))
				Expect(resourceSource).To(Equal(atc.Source{"custom": "some-secret-sauce"}))
				Expect(resourceTypes).To(Equal(creds.VersionedResourceTypes{}))

				Expect(fakeResourceType.SetCheckErrorCallCount()).To(Equal(1))
				err := fakeResourceType.SetCheckErrorArgsForCall(0)
				Expect(err).To(BeNil())

				Expect(fakeResourceType.SetResourceConfigCallCount()).To(Equal(1))
				resourceConfigID := fakeResourceType.SetResourceConfigArgsForCall(0)
				Expect(resourceConfigID).To(Equal(123))

				Expect(fakeResourceFactory.NewResourceCallCount()).To(Equal(1))
				_, _, owner, metadata, containerSpec, workerSpec, resourceTypes, _ := fakeResourceFactory.NewResourceArgsForCall(0)
				Expect(owner).To(Equal(db.NewResourceConfigCheckSessionContainerOwner(fakeResourceConfig, ContainerExpiries)))
				Expect(metadata).To(Equal(db.ContainerMetadata{
					Type: db.ContainerTypeCheck,
				}))
				Expect(containerSpec).To(Equal(worker.ContainerSpec{
					ImageSpec: worker.ImageSpec{
						ResourceType: "registry-image",
					},
					Tags:   []string{"some-tag"},
					TeamID: 123,
				}))
				Expect(workerSpec).To(Equal(worker.WorkerSpec{
					ResourceType:  "registry-image",
					Tags:          []string{"some-tag"},
					ResourceTypes: creds.VersionedResourceTypes{},
				}))
				Expect(resourceTypes).To(Equal(creds.VersionedResourceTypes{}))
			})

			Context("when the resource type depends on another custom type", func() {
				var otherResourceType *dbfakes.FakeResourceType

				BeforeEach(func() {
					otherResourceType = new(dbfakes.FakeResourceType)
					otherResourceType.IDReturns(39)
					otherResourceType.NameReturns("custom-resource-parent")
					otherResourceType.TypeReturns("registry-image")

					fakeResourceType = new(dbfakes.FakeResourceType)
					fakeResourceType.IDReturns(40)
					fakeResourceType.NameReturns("custom-resource-child")
					fakeResourceType.TypeReturns("custom-resource-parent")
					fakeResourceType.SetResourceConfigReturns(nil)

					// testing recursion is fun!
					fakeDBPipeline.ResourceTypesReturnsOnCall(0, []db.ResourceType{
						fakeResourceType,
						otherResourceType,
					}, nil)
					fakeDBPipeline.ResourceTypeReturnsOnCall(0, fakeResourceType, true, nil)
				})

				Context("when the custom type does not yet have a version", func() {
					BeforeEach(func() {
						otherResourceType.VersionReturns(nil)
					})

					It("checks for versions of the parent resource type", func() {
						By("calling .scan() for the resource type name")
						Expect(fakeDBPipeline.ResourceTypeCallCount()).To(Equal(2))
						Expect(fakeDBPipeline.ResourceTypeArgsForCall(1)).To(Equal("custom-resource-parent"))
					})

					Context("when the check for the parent succeeds", func() {
						It("reloads the resource types from the database", func() {

							Expect(runErr).ToNot(HaveOccurred())
							Expect(fakeDBPipeline.ResourceTypesCallCount()).To(Equal(4))
						})
					})

					Context("somethinng fails in the parent resource scan", func() {
						var parentResourceTypeErr = errors.New("jma says no recursion in production")
						BeforeEach(func() {
							fakeDBPipeline.ResourceTypeReturnsOnCall(1, otherResourceType, true, parentResourceTypeErr)
						})

						It("returns the error from scanning the parent", func() {
							Expect(runErr).To(Equal(parentResourceTypeErr))
						})

						It("saves the error to check_error on resource type row in db", func() {
							Expect(fakeResourceType.SetCheckErrorCallCount()).To(Equal(1))

							err := fakeResourceType.SetCheckErrorArgsForCall(0)
							Expect(err).To(HaveOccurred())
							Expect(err.Error()).To(Equal("jma says no recursion in production"))
						})
					})
				})

				Context("when the custom type has a version already", func() {
					BeforeEach(func() {
						otherResourceType.VersionReturns(atc.Version{"custom": "image-version"})
					})

					It("does not check for versions of the parent resource type", func() {
						Expect(fakeDBPipeline.ResourceTypeCallCount()).To(Equal(1))
						Expect(fakeResourceConfigFactory.FindOrCreateResourceConfigCallCount()).To(Equal(1))
					})
				})
			})

			Context("when the resource type overrides a base resource type", func() {
				BeforeEach(func() {
					otherResourceType := fakeResourceType

					fakeResourceType = new(dbfakes.FakeResourceType)
					fakeResourceType.IDReturns(40)
					fakeResourceType.NameReturns("registry-image")
					fakeResourceType.TypeReturns("registry-image")
					fakeResourceType.SourceReturns(atc.Source{"custom": "((source-params))"})

					fakeDBPipeline.ResourceTypesReturns([]db.ResourceType{
						fakeResourceType,
						otherResourceType,
					}, nil)
					fakeDBPipeline.ResourceTypeReturns(fakeResourceType, true, nil)
					fakeResourceType.SetResourceConfigReturns(nil)
				})

				It("constructs the resource of the correct type", func() {
					Expect(fakeResourceConfigFactory.FindOrCreateResourceConfigCallCount()).To(Equal(1))
					_, resourceType, resourceSource, resourceTypes := fakeResourceConfigFactory.FindOrCreateResourceConfigArgsForCall(0)
					Expect(resourceType).To(Equal("registry-image"))
					Expect(resourceSource).To(Equal(atc.Source{"custom": "some-secret-sauce"}))
					Expect(resourceTypes).To(Equal(creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
						versionedResourceType,
					})))

					Expect(fakeResourceType.SetResourceConfigCallCount()).To(Equal(1))
					resourceConfigID := fakeResourceType.SetResourceConfigArgsForCall(0)
					Expect(resourceConfigID).To(Equal(123))

					Expect(fakeResourceFactory.NewResourceCallCount()).To(Equal(1))
					_, _, owner, metadata, containerSpec, workerSpec, resourceTypes, _ := fakeResourceFactory.NewResourceArgsForCall(0)
					Expect(owner).To(Equal(db.NewResourceConfigCheckSessionContainerOwner(fakeResourceConfig, ContainerExpiries)))
					Expect(metadata).To(Equal(db.ContainerMetadata{
						Type: db.ContainerTypeCheck,
					}))
					Expect(containerSpec).To(Equal(worker.ContainerSpec{
						ImageSpec: worker.ImageSpec{
							ResourceType: "registry-image",
						},
						TeamID: 123,
					}))
					Expect(workerSpec).To(Equal(worker.WorkerSpec{
						ResourceType:  "registry-image",
						ResourceTypes: creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{versionedResourceType}),
					}))
					Expect(resourceTypes).To(Equal(creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
						versionedResourceType,
					})))
				})
			})

			It("grabs an immediate resource checking lock before checking, breaks lock after done", func() {
				Expect(fakeResourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheckCallCount()).To(Equal(1))

				_, leaseInterval, immediate := fakeResourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheckArgsForCall(0)
				Expect(leaseInterval).To(Equal(interval))
				Expect(immediate).To(BeTrue())

				Eventually(fakeLock.ReleaseCallCount()).Should(Equal(1))
			})

			Context("when creating the resource config fails", func() {
				BeforeEach(func() {
					fakeResourceConfigFactory.FindOrCreateResourceConfigReturns(nil, errors.New("catastrophe"))
				})

				It("sets the check error and returns the error", func() {
					Expect(runErr).To(HaveOccurred())
					Expect(fakeResourceType.SetCheckErrorCallCount()).To(Equal(1))

					chkErr := fakeResourceType.SetCheckErrorArgsForCall(0)
					Expect(chkErr).To(MatchError("catastrophe"))
				})
			})

			Context("when updating the resource config id on the resource type fails", func() {
				BeforeEach(func() {
					fakeResourceType.SetResourceConfigReturns(errors.New("catastrophe"))
				})

				It("sets the check error and returns the error", func() {
					Expect(runErr).To(HaveOccurred())
					Expect(fakeResourceConfig.SetCheckErrorCallCount()).To(Equal(1))

					chkErr := fakeResourceConfig.SetCheckErrorArgsForCall(0)
					Expect(chkErr).To(MatchError("catastrophe"))
				})
			})

			Context("when creating the resource checker fails", func() {
				BeforeEach(func() {
					fakeResourceFactory.NewResourceReturns(nil, errors.New("catastrophe"))
				})

				It("sets the check error and returns the error", func() {
					Expect(runErr).To(HaveOccurred())
					Expect(fakeResourceConfig.SetCheckErrorCallCount()).To(Equal(1))

					chkErr := fakeResourceConfig.SetCheckErrorArgsForCall(0)
					Expect(chkErr).To(MatchError("catastrophe"))
				})
			})

			Context("when creating the resource checker fails with no global workers", func() {
				BeforeEach(func() {
					fakeResourceFactory.NewResourceReturns(nil, worker.ErrNoGlobalWorkers)
				})

				It("sets the check error and returns no workers error", func() {
					Expect(runErr).To(HaveOccurred())
					Expect(fakeResourceConfig.SetCheckErrorCallCount()).To(Equal(1))

					resourceErr := fakeResourceConfig.SetCheckErrorArgsForCall(0)
					Expect(resourceErr).To(Equal(atc.ErrNoWorkers))
				})
			})

			Context("when there is no current version", func() {
				BeforeEach(func() {
					fakeResourceType.VersionReturns(nil)
				})

				It("checks from nil", func() {
					_, _, version := fakeResource.CheckArgsForCall(0)
					Expect(version).To(BeNil())
				})
			})

			Context("when there is a current version", func() {
				BeforeEach(func() {
					fakeResourceConfigVersion := new(dbfakes.FakeResourceConfigVersion)
					fakeResourceConfigVersion.IDReturns(1)
					fakeResourceConfigVersion.VersionReturns(db.Version{"version": "42"})

					fakeResourceConfig.LatestVersionReturns(fakeResourceConfigVersion, true, nil)
				})

				It("checks with it", func() {
					Expect(fakeResource.CheckCallCount()).To(Equal(1))
					_, _, version := fakeResource.CheckArgsForCall(0)
					Expect(version).To(Equal(atc.Version{"version": "42"}))
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

						Expect(source).To(Equal(atc.Source{"custom": "some-secret-sauce"}))

						checkedFrom <- from
						result := checkResults[check]
						check++

						return result, nil
					}
				})

				It("saves all resource type versions", func() {
					Eventually(fakeResourceConfig.SaveVersionsCallCount).Should(Equal(1))

					version := fakeResourceConfig.SaveVersionsArgsForCall(0)
					Expect(version).To(Equal(nextVersions))
				})
			})

			Context("when checking fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeResource.CheckReturns(nil, disaster)
				})

				It("exits with the failure", func() {
					Expect(runErr).To(HaveOccurred())
					Expect(runErr).To(Equal(disaster))
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

			Context("when the pipeline is paused", func() {
				BeforeEach(func() {
					fakeDBPipeline.CheckPausedReturns(true, nil)
				})

				It("does not check", func() {
					Expect(fakeResource.CheckCallCount()).To(BeZero())
				})

				It("does not return an error", func() {
					Expect(runErr).NotTo(HaveOccurred())
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
			scanErr = scanner.ScanFromVersion(lagertest.NewTestLogger("test"), "some-resource-type", fromVersion)
		})

		Context("if the lock can be acquired", func() {
			BeforeEach(func() {
				fakeResourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheckReturns(fakeLock, true, nil)
				fakeResourceConfigVersion := new(dbfakes.FakeResourceConfigVersion)
				fakeResourceConfigVersion.IDReturns(1)
				fakeResourceConfigVersion.VersionReturns(db.Version{"custom": "version"})

				fakeResourceConfig.LatestVersionReturns(fakeResourceConfigVersion, true, nil)
			})

			Context("when fromVersion is nil", func() {
				It("checks from the current version", func() {
					_, _, version := fakeResource.CheckArgsForCall(0)
					Expect(version).To(Equal(atc.Version{"custom": "version"}))
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
						Expect(versions[0]).To(Equal(fromVersion))
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
					fakeDBPipeline.ResourceTypeReturns(nil, false, nil)
				})

				It("returns an error", func() {
					Expect(scanErr).To(HaveOccurred())
					Expect(scanErr.Error()).To(ContainSubstring("resource type not found: some-resource-type"))
				})
			})
		})
	})
})
