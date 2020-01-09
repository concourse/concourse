package radar_test

import (
	"context"
	"errors"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds/credsfakes"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	. "github.com/concourse/concourse/atc/radar"
	rfakes "github.com/concourse/concourse/atc/resource/resourcefakes"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	"github.com/concourse/concourse/vars"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceTypeScanner", func() {
	var (
		epoch time.Time

		fakeContainer             *workerfakes.FakeContainer
		fakeWorker                *workerfakes.FakeWorker
		fakePool                  *workerfakes.FakePool
		fakeStrategy              *workerfakes.FakeContainerPlacementStrategy
		fakeResourceFactory       *rfakes.FakeResourceFactory
		fakeResourceConfigFactory *dbfakes.FakeResourceConfigFactory
		fakeDBPipeline            *dbfakes.FakePipeline
		fakeResourceConfig        *dbfakes.FakeResourceConfig
		fakeResourceConfigScope   *dbfakes.FakeResourceConfigScope
		fakeClock                 *fakeclock.FakeClock
		fakeVarSourcePool         *credsfakes.FakeVarSourcePool
		fakeSecrets               *credsfakes.FakeSecrets
		interval                  time.Duration
		variables                 vars.Variables
		metadata                  db.ContainerMetadata

		fakeResourceType          *dbfakes.FakeResourceType
		interpolatedResourceTypes atc.VersionedResourceTypes

		scanner Scanner

		fakeLock *lockfakes.FakeLock
		teamID   = 123
	)

	BeforeEach(func() {
		fakeLock = &lockfakes.FakeLock{}
		interval = 1 * time.Minute

		fakeSecrets = new(credsfakes.FakeSecrets)
		fakeSecrets.GetStub = func(key string) (interface{}, *time.Time, bool, error) {
			if key == "source-params" {
				return "some-secret-sauce", nil, true, nil
			}
			return nil, nil, false, nil
		}

		variables = vars.StaticVariables{
			"source-params": "some-secret-sauce",
		}

		interpolatedResourceTypes = atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Name:   "some-custom-resource",
					Type:   "registry-image",
					Source: atc.Source{"custom": "some-secret-sauce"},
					Tags:   atc.Tags{"some-tag"},
				},
				Version: atc.Version{"custom": "version"},
			},
		}

		fakeClock = fakeclock.NewFakeClock(epoch)
		fakeVarSourcePool = new(credsfakes.FakeVarSourcePool)
		fakeContainer = new(workerfakes.FakeContainer)
		fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)
		fakePool = new(workerfakes.FakePool)
		fakeWorker = new(workerfakes.FakeWorker)
		fakeResourceFactory = new(rfakes.FakeResourceFactory)
		fakeResourceConfigFactory = new(dbfakes.FakeResourceConfigFactory)
		fakeResourceType = new(dbfakes.FakeResourceType)
		fakeDBPipeline = new(dbfakes.FakePipeline)
		fakeResourceConfig = new(dbfakes.FakeResourceConfig)
		fakeResourceConfig.IDReturns(123)
		fakeResourceConfig.OriginBaseResourceTypeReturns(&db.UsedBaseResourceType{ID: 456})
		fakeResourceConfigScope = new(dbfakes.FakeResourceConfigScope)
		fakeResourceConfigScope.IDReturns(123)
		fakeResourceConfigScope.ResourceConfigReturns(fakeResourceConfig)

		fakeResourceType.IDReturns(39)
		fakeResourceType.NameReturns("some-custom-resource")
		fakeResourceType.TypeReturns("registry-image")
		fakeResourceType.SourceReturns(atc.Source{"custom": "((source-params))"})
		fakeResourceType.VersionReturns(atc.Version{"custom": "version"})
		fakeResourceType.TagsReturns(atc.Tags{"some-tag"})
		fakeResourceType.SetResourceConfigReturns(fakeResourceConfigScope, nil)

		fakeDBPipeline.IDReturns(42)
		fakeDBPipeline.NameReturns("some-pipeline")
		fakeDBPipeline.TeamIDReturns(teamID)
		fakeDBPipeline.ReloadReturns(true, nil)
		fakeDBPipeline.ResourceTypesReturns([]db.ResourceType{fakeResourceType}, nil)
		fakeDBPipeline.ResourceTypeByIDReturns(fakeResourceType, true, nil)
		fakeDBPipeline.VariablesReturns(variables, nil)

		scanner = NewResourceTypeScanner(
			fakeClock,
			fakePool,
			fakeResourceFactory,
			fakeResourceConfigFactory,
			interval,
			fakeDBPipeline,
			"https://www.example.com",
			fakeSecrets,
			fakeVarSourcePool,
			fakeStrategy,
		)
	})

	Describe("Run", func() {
		var (
			fakeResource   *rfakes.FakeResource
			actualInterval time.Duration
			runErr         error
		)

		BeforeEach(func() {
			fakeWorker.NameReturns("some-worker")
			fakePool.FindOrChooseWorkerForContainerReturns(fakeWorker, nil)

			fakeContainer.HandleReturns("some-handle")
			fakeWorker.FindOrCreateContainerReturns(fakeContainer, nil)

			fakeResource = new(rfakes.FakeResource)
			fakeResourceFactory.NewResourceReturns(fakeResource)
		})

		JustBeforeEach(func() {
			actualInterval, runErr = scanner.Run(lagertest.NewTestLogger("test"), fakeResourceType.ID())
		})

		Context("when the lock cannot be acquired", func() {
			BeforeEach(func() {
				fakeResourceConfigScope.AcquireResourceCheckingLockReturns(nil, false, nil)
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
				fakeResourceConfigScope.AcquireResourceCheckingLockReturns(fakeLock, true, nil)
			})

			Context("when the last checked was not updated", func() {
				BeforeEach(func() {
					fakeResourceConfigScope.UpdateLastCheckStartTimeReturns(false, nil)
				})

				It("does not check", func() {
					Expect(fakeResource.CheckCallCount()).To(Equal(0))
				})

				It("returns the configured interval", func() {
					Expect(runErr).To(Equal(ErrFailedToAcquireLock))
					Expect(actualInterval).To(Equal(interval))
				})
			})

			Context("when the last checked was updated", func() {
				BeforeEach(func() {
					fakeResourceConfigScope.UpdateLastCheckStartTimeReturns(true, nil)
				})

				It("checks immediately", func() {
					Expect(fakeResource.CheckCallCount()).To(Equal(1))
				})

				It("constructs the resource of the correct type", func() {
					Expect(fakeResourceType.SetResourceConfigCallCount()).To(Equal(1))
					resourceSource, resourceTypes := fakeResourceType.SetResourceConfigArgsForCall(0)
					Expect(resourceSource).To(Equal(atc.Source{"custom": "some-secret-sauce"}))
					Expect(resourceTypes).To(Equal(atc.VersionedResourceTypes{}))

					_, _, owner, containerSpec, workerSpec, _ := fakePool.FindOrChooseWorkerForContainerArgsForCall(0)
					Expect(owner).To(Equal(db.NewResourceConfigCheckSessionContainerOwner(123, 456, ContainerExpiries)))
					Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
						ResourceType: "registry-image",
					}))
					Expect(containerSpec.Tags).To(Equal([]string{"some-tag"}))
					Expect(containerSpec.TeamID).To(Equal(123))
					Expect(workerSpec).To(Equal(worker.WorkerSpec{
						ResourceType:  "registry-image",
						Tags:          []string{"some-tag"},
						ResourceTypes: atc.VersionedResourceTypes{},
						TeamID:        123,
					}))

					Expect(fakeWorker.FindOrCreateContainerCallCount()).To(Equal(1))

					_, _, _, owner, metadata, containerSpec, resourceTypes = fakeWorker.FindOrCreateContainerArgsForCall(0)
					Expect(owner).To(Equal(db.NewResourceConfigCheckSessionContainerOwner(123, 456, ContainerExpiries)))
					Expect(metadata).To(Equal(db.ContainerMetadata{
						Type: db.ContainerTypeCheck,
					}))
					Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
						ResourceType: "registry-image",
					}))
					Expect(containerSpec.Tags).To(Equal([]string{"some-tag"}))
					Expect(containerSpec.TeamID).To(Equal(123))
					Expect(resourceTypes).To(Equal(atc.VersionedResourceTypes{}))
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
						fakeResourceType.SetResourceConfigReturns(fakeResourceConfigScope, nil)

						fakeDBPipeline.ResourceTypesReturns([]db.ResourceType{
							fakeResourceType,
							otherResourceType,
						}, nil)
						fakeDBPipeline.ResourceTypeByIDReturns(fakeResourceType, true, nil)
					})

					It("constructs the resource of the correct type", func() {
						Expect(fakeResourceType.SetResourceConfigCallCount()).To(Equal(1))
						resourceSource, resourceTypes := fakeResourceType.SetResourceConfigArgsForCall(0)
						Expect(resourceSource).To(Equal(atc.Source{"custom": "some-secret-sauce"}))
						Expect(resourceTypes).To(Equal(interpolatedResourceTypes))

						Expect(fakeResourceType.SetCheckSetupErrorCallCount()).To(Equal(1))
						err := fakeResourceType.SetCheckSetupErrorArgsForCall(0)
						Expect(err).To(BeNil())

						_, _, owner, containerSpec, workerSpec, _ := fakePool.FindOrChooseWorkerForContainerArgsForCall(0)
						Expect(owner).To(Equal(db.NewResourceConfigCheckSessionContainerOwner(123, 456, ContainerExpiries)))
						Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
							ResourceType: "registry-image",
						}))
						Expect(containerSpec.TeamID).To(Equal(123))
						Expect(workerSpec).To(Equal(worker.WorkerSpec{
							ResourceType:  "registry-image",
							ResourceTypes: interpolatedResourceTypes,
							TeamID:        123,
						}))

						Expect(fakeWorker.FindOrCreateContainerCallCount()).To(Equal(1))
						_, _, _, owner, metadata, containerSpec, resourceTypes = fakeWorker.FindOrCreateContainerArgsForCall(0)
						Expect(owner).To(Equal(db.NewResourceConfigCheckSessionContainerOwner(123, 456, ContainerExpiries)))
						Expect(metadata).To(Equal(db.ContainerMetadata{
							Type: db.ContainerTypeCheck,
						}))
						Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
							ResourceType: "registry-image",
						}))
						Expect(containerSpec.TeamID).To(Equal(123))
						Expect(resourceTypes).To(Equal(interpolatedResourceTypes))
					})
				})

				Context("when the resource type config has a specified check interval", func() {
					BeforeEach(func() {
						fakeResourceType.CheckEveryReturns("10ms")
						fakeDBPipeline.ResourceTypeByIDReturns(fakeResourceType, true, nil)
					})

					It("leases for the configured interval", func() {
						Expect(fakeResourceConfigScope.AcquireResourceCheckingLockCallCount()).To(Equal(1))
						Expect(fakeResourceConfigScope.UpdateLastCheckStartTimeCallCount()).To(Equal(1))

						leaseInterval, immediate := fakeResourceConfigScope.UpdateLastCheckStartTimeArgsForCall(0)
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
							fakeDBPipeline.ResourceTypeByIDReturns(fakeResourceType, true, nil)
						})

						It("sets the check error", func() {
							Expect(fakeResourceType.SetCheckSetupErrorCallCount()).To(Equal(1))

							resourceErr := fakeResourceType.SetCheckSetupErrorArgsForCall(0)
							Expect(resourceErr).To(MatchError("time: invalid duration bad-value"))
						})

						It("returns an error", func() {
							Expect(runErr).To(HaveOccurred())
						})
					})
				})

				It("grabs a periodic resource checking lock before checking, breaks lock after done", func() {
					Expect(fakeResourceConfigScope.AcquireResourceCheckingLockCallCount()).To(Equal(1))
					Expect(fakeResourceConfigScope.UpdateLastCheckStartTimeCallCount()).To(Equal(1))

					leaseInterval, immediate := fakeResourceConfigScope.UpdateLastCheckStartTimeArgsForCall(0)
					Expect(leaseInterval).To(Equal(interval))
					Expect(immediate).To(BeFalse())

					Eventually(fakeLock.ReleaseCallCount()).Should(Equal(1))
				})

				It("invokes resourceFactory.NewResource with the correct arguments", func() {
					actualSource, actualParams, actualVersion := fakeResourceFactory.NewResourceArgsForCall(0)
					Expect(actualSource).To(Equal(atc.Source{"custom": "some-secret-sauce"}))
					Expect(actualParams).To(BeNil())
					Expect(actualVersion).To(BeNil())
				})

				Context("when there is no current version", func() {
					BeforeEach(func() {
						fakeResourceType.VersionReturns(nil)
					})

					It("checks from nil", func() {
						_, _, version := fakeResourceFactory.NewResourceArgsForCall(0)
						Expect(version).To(BeNil())
					})
				})

				Context("when there is a current version", func() {
					BeforeEach(func() {
						fakeResourceConfigVersion := new(dbfakes.FakeResourceConfigVersion)
						fakeResourceConfigVersion.IDReturns(1)
						fakeResourceConfigVersion.VersionReturns(db.Version{"version": "42"})

						fakeResourceConfigScope.LatestVersionReturns(fakeResourceConfigVersion, true, nil)
					})

					It("checks with it", func() {
						Expect(fakeResource.CheckCallCount()).To(Equal(1))
						_, _, version := fakeResourceFactory.NewResourceArgsForCall(0)
						Expect(version).To(Equal(atc.Version{"version": "42"}))
					})
				})

				Context("when the check returns versions", func() {
					var nextVersions []atc.Version

					BeforeEach(func() {
						nextVersions = []atc.Version{
							{"version": "1"},
							{"version": "2"},
							{"version": "3"},
						}

						checkResults := map[int][]atc.Version{
							0: nextVersions,
						}

						check := 0
						fakeResource.CheckStub = func(ctx context.Context, processSpec runtime.ProcessSpec, runner runtime.Runner) ([]atc.Version, error) {
							defer GinkgoRecover()

							Expect(processSpec).To(Equal(runtime.ProcessSpec{Path: "/opt/resource/check"}))
							Expect(runner).To(Equal(fakeContainer))

							result := checkResults[check]
							check++

							return result, nil
						}
					})

					It("saves all resource type versions", func() {
						Eventually(fakeResourceConfigScope.SaveVersionsCallCount).Should(Equal(1))

						version := fakeResourceConfigScope.SaveVersionsArgsForCall(0)
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
						Expect(fakeResourceConfigScope.SetCheckErrorCallCount()).To(Equal(1))

						err := fakeResourceConfigScope.SetCheckErrorArgsForCall(0)
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
	})

	Describe("Scan", func() {
		var (
			fakeResource *rfakes.FakeResource
			runErr       error
		)

		BeforeEach(func() {
			fakeWorker.NameReturns("some-worker")
			fakePool.FindOrChooseWorkerForContainerReturns(fakeWorker, nil)

			fakeContainer.HandleReturns("some-handle")
			fakeWorker.FindOrCreateContainerReturns(fakeContainer, nil)

			fakeResource = new(rfakes.FakeResource)
			fakeResourceFactory.NewResourceReturns(fakeResource)
		})

		JustBeforeEach(func() {
			runErr = scanner.Scan(lagertest.NewTestLogger("test"), fakeResourceType.ID())
		})

		Context("when the lock can be acquired and last checked is updated", func() {
			BeforeEach(func() {
				fakeResourceConfigScope.AcquireResourceCheckingLockReturns(fakeLock, true, nil)
				fakeResourceConfigScope.UpdateLastCheckStartTimeReturns(true, nil)
			})

			It("checks immediately", func() {
				Expect(fakeResource.CheckCallCount()).To(Equal(1))
			})

			It("constructs the resource of the correct type", func() {
				Expect(fakeResourceType.SetResourceConfigCallCount()).To(Equal(1))
				resourceSource, resourceTypes := fakeResourceType.SetResourceConfigArgsForCall(0)
				Expect(resourceSource).To(Equal(atc.Source{"custom": "some-secret-sauce"}))
				Expect(resourceTypes).To(Equal(atc.VersionedResourceTypes{}))

				Expect(fakeResourceType.SetCheckSetupErrorCallCount()).To(Equal(1))
				err := fakeResourceType.SetCheckSetupErrorArgsForCall(0)
				Expect(err).To(BeNil())

				_, _, owner, containerSpec, workerSpec, _ := fakePool.FindOrChooseWorkerForContainerArgsForCall(0)
				Expect(owner).To(Equal(db.NewResourceConfigCheckSessionContainerOwner(123, 456, ContainerExpiries)))
				Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
					ResourceType: "registry-image",
				}))
				Expect(containerSpec.Tags).To(Equal([]string{"some-tag"}))
				Expect(containerSpec.TeamID).To(Equal(123))
				Expect(workerSpec).To(Equal(worker.WorkerSpec{
					ResourceType:  "registry-image",
					Tags:          []string{"some-tag"},
					ResourceTypes: atc.VersionedResourceTypes{},
					TeamID:        123,
				}))

				Expect(fakeWorker.FindOrCreateContainerCallCount()).To(Equal(1))
				_, _, _, owner, metadata, containerSpec, resourceTypes = fakeWorker.FindOrCreateContainerArgsForCall(0)
				Expect(owner).To(Equal(db.NewResourceConfigCheckSessionContainerOwner(123, 456, ContainerExpiries)))
				Expect(metadata).To(Equal(db.ContainerMetadata{
					Type: db.ContainerTypeCheck,
				}))
				Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
					ResourceType: "registry-image",
				}))
				Expect(containerSpec.Tags).To(Equal([]string{"some-tag"}))
				Expect(containerSpec.TeamID).To(Equal(123))
				Expect(resourceTypes).To(Equal(atc.VersionedResourceTypes{}))
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
					fakeResourceType.SetResourceConfigReturns(fakeResourceConfigScope, nil)

					// testing recursion is fun!
					fakeDBPipeline.ResourceTypesReturnsOnCall(0, []db.ResourceType{
						fakeResourceType,
						otherResourceType,
					}, nil)
					fakeDBPipeline.ResourceTypeByIDReturnsOnCall(0, fakeResourceType, true, nil)
				})

				Context("when the custom type does not yet have a version", func() {
					BeforeEach(func() {
						otherResourceType.VersionReturns(nil)
					})

					It("checks for versions of the parent resource type", func() {
						By("calling .scan() for the resource type name")
						Expect(fakeDBPipeline.ResourceTypeByIDCallCount()).To(Equal(2))
						Expect(fakeDBPipeline.ResourceTypeByIDArgsForCall(1)).To(Equal(39))
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
							fakeDBPipeline.ResourceTypeByIDReturnsOnCall(1, otherResourceType, true, parentResourceTypeErr)
						})

						It("returns the error from scanning the parent", func() {
							Expect(runErr).To(Equal(parentResourceTypeErr))
						})

						It("saves the error to check_error on resource type row in db", func() {
							Expect(fakeResourceType.SetCheckSetupErrorCallCount()).To(Equal(1))

							err := fakeResourceType.SetCheckSetupErrorArgsForCall(0)
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
						Expect(fakeDBPipeline.ResourceTypeByIDCallCount()).To(Equal(1))
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
					fakeResourceType.SetResourceConfigReturns(fakeResourceConfigScope, nil)

					fakeDBPipeline.ResourceTypesReturns([]db.ResourceType{
						fakeResourceType,
						otherResourceType,
					}, nil)
					fakeDBPipeline.ResourceTypeByIDReturns(fakeResourceType, true, nil)
				})

				It("constructs the resource of the correct type", func() {
					Expect(fakeResourceType.SetResourceConfigCallCount()).To(Equal(1))
					resourceSource, resourceTypes := fakeResourceType.SetResourceConfigArgsForCall(0)
					Expect(resourceSource).To(Equal(atc.Source{"custom": "some-secret-sauce"}))
					Expect(resourceTypes).To(Equal(interpolatedResourceTypes))

					_, _, owner, containerSpec, workerSpec, _ := fakePool.FindOrChooseWorkerForContainerArgsForCall(0)
					Expect(owner).To(Equal(db.NewResourceConfigCheckSessionContainerOwner(123, 456, ContainerExpiries)))
					Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
						ResourceType: "registry-image",
					}))
					Expect(containerSpec.TeamID).To(Equal(123))
					Expect(workerSpec).To(Equal(worker.WorkerSpec{
						ResourceType:  "registry-image",
						ResourceTypes: interpolatedResourceTypes,
						TeamID:        123,
					}))

					Expect(fakeWorker.FindOrCreateContainerCallCount()).To(Equal(1))
					_, _, _, owner, metadata, containerSpec, resourceTypes = fakeWorker.FindOrCreateContainerArgsForCall(0)
					Expect(owner).To(Equal(db.NewResourceConfigCheckSessionContainerOwner(123, 456, ContainerExpiries)))
					Expect(metadata).To(Equal(db.ContainerMetadata{
						Type: db.ContainerTypeCheck,
					}))
					Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
						ResourceType: "registry-image",
					}))
					Expect(containerSpec.TeamID).To(Equal(123))
					Expect(resourceTypes).To(Equal(interpolatedResourceTypes))
				})
			})

			It("grabs an immediate resource checking lock before checking, breaks lock after done", func() {
				Expect(fakeResourceConfigScope.AcquireResourceCheckingLockCallCount()).To(Equal(1))
				Expect(fakeResourceConfigScope.UpdateLastCheckStartTimeCallCount()).To(Equal(1))

				leaseInterval, immediate := fakeResourceConfigScope.UpdateLastCheckStartTimeArgsForCall(0)
				Expect(leaseInterval).To(Equal(interval))
				Expect(immediate).To(BeTrue())

				Eventually(fakeLock.ReleaseCallCount()).Should(Equal(1))
			})

			Context("when setting the resource config on the resource type fails", func() {
				BeforeEach(func() {
					fakeResourceType.SetResourceConfigReturns(nil, errors.New("catastrophe"))
				})

				It("sets the check error and returns the error", func() {
					Expect(runErr).To(HaveOccurred())
					Expect(fakeResourceType.SetCheckSetupErrorCallCount()).To(Equal(1))

					chkErr := fakeResourceType.SetCheckSetupErrorArgsForCall(0)
					Expect(chkErr).To(MatchError("catastrophe"))
				})
			})

			Context("when creating the container fails", func() {
				BeforeEach(func() {
					fakeWorker.FindOrCreateContainerReturns(nil, errors.New("catastrophe"))
				})

				It("sets the check error and returns the error", func() {
					Expect(runErr).To(HaveOccurred())
					Expect(fakeResourceConfigScope.SetCheckErrorCallCount()).To(Equal(1))

					chkErr := fakeResourceConfigScope.SetCheckErrorArgsForCall(0)
					Expect(chkErr).To(MatchError("catastrophe"))
				})
			})

			Context("when finding or choosing the worker fails", func() {
				BeforeEach(func() {
					fakePool.FindOrChooseWorkerForContainerReturns(nil, errors.New("catastrophe"))
				})

				It("sets the check error and returns the error", func() {
					Expect(runErr).To(HaveOccurred())
					Expect(fakeResourceConfigScope.SetCheckErrorCallCount()).To(Equal(1))

					chkErr := fakeResourceConfigScope.SetCheckErrorArgsForCall(0)
					Expect(chkErr).To(MatchError("catastrophe"))
				})
			})

			Context("when there is no current version", func() {
				BeforeEach(func() {
					fakeResourceType.VersionReturns(nil)
				})

				It("checks from nil", func() {
					_, _, version := fakeResourceFactory.NewResourceArgsForCall(0)
					Expect(version).To(BeNil())
				})
			})

			Context("when there is a current version", func() {
				BeforeEach(func() {
					fakeResourceConfigVersion := new(dbfakes.FakeResourceConfigVersion)
					fakeResourceConfigVersion.IDReturns(1)
					fakeResourceConfigVersion.VersionReturns(db.Version{"version": "42"})

					fakeResourceConfigScope.LatestVersionReturns(fakeResourceConfigVersion, true, nil)
				})

				It("checks with it", func() {
					Expect(fakeResource.CheckCallCount()).To(Equal(1))
					_, _, version := fakeResourceFactory.NewResourceArgsForCall(0)
					Expect(version).To(Equal(atc.Version{"version": "42"}))
				})
			})

			Context("when the check returns versions", func() {
				var nextVersions []atc.Version

				BeforeEach(func() {
					nextVersions = []atc.Version{
						{"version": "1"},
						{"version": "2"},
						{"version": "3"},
					}

					checkResults := map[int][]atc.Version{
						0: nextVersions,
					}

					check := 0
					fakeResource.CheckStub = func(ctx context.Context, processSpec runtime.ProcessSpec, runner runtime.Runner) ([]atc.Version, error) {
						defer GinkgoRecover()

						Expect(processSpec).To(Equal(runtime.ProcessSpec{Path: "/opt/resource/check"}))
						Expect(runner).To(Equal(fakeContainer))

						result := checkResults[check]
						check++

						return result, nil
					}
				})

				It("saves all resource type versions", func() {
					Eventually(fakeResourceConfigScope.SaveVersionsCallCount).Should(Equal(1))

					version := fakeResourceConfigScope.SaveVersionsArgsForCall(0)
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

					fakeResourceConfigScope.AcquireResourceCheckingLockStub = func(logger lager.Logger) (lock.Lock, bool, error) {
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
					Expect(fakeResourceConfigScope.AcquireResourceCheckingLockCallCount()).To(Equal(3))
					Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
				})
			})

			Context("when last checked interval is not immediately updated", func() {
				BeforeEach(func() {
					results := make(chan bool, 4)
					results <- false
					results <- false
					results <- true
					results <- true
					close(results)

					fakeResourceConfigScope.UpdateLastCheckStartTimeStub = func(interval time.Duration, immediate bool) (bool, error) {
						if <-results {
							return true, nil
						} else {
							// allow the sleep to continue
							go fakeClock.WaitForWatcherAndIncrement(time.Second)
							return false, nil
						}
					}
				})

				It("retries every second until it is", func() {
					Expect(fakeResourceConfigScope.AcquireResourceCheckingLockCallCount()).To(Equal(3))
					Expect(fakeResourceConfigScope.UpdateLastCheckStartTimeCallCount()).To(Equal(3))

					leaseInterval, immediate := fakeResourceConfigScope.UpdateLastCheckStartTimeArgsForCall(0)
					Expect(leaseInterval).To(Equal(interval))
					Expect(immediate).To(BeTrue())

					leaseInterval, immediate = fakeResourceConfigScope.UpdateLastCheckStartTimeArgsForCall(1)
					Expect(leaseInterval).To(Equal(interval))
					Expect(immediate).To(BeTrue())

					leaseInterval, immediate = fakeResourceConfigScope.UpdateLastCheckStartTimeArgsForCall(2)
					Expect(leaseInterval).To(Equal(interval))
					Expect(immediate).To(BeTrue())

					Expect(fakeLock.ReleaseCallCount()).To(Equal(3))
				})
			})

			It("clears the resource's check error", func() {
				Expect(fakeResourceConfigScope.SetCheckErrorCallCount()).To(Equal(1))

				err := fakeResourceConfigScope.SetCheckErrorArgsForCall(0)
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
			fakeWorker.NameReturns("some-worker")
			fakePool.FindOrChooseWorkerForContainerReturns(fakeWorker, nil)

			fakeContainer.HandleReturns("some-handle")
			fakeWorker.FindOrCreateContainerReturns(fakeContainer, nil)

			fakeResource = new(rfakes.FakeResource)
			fakeResourceFactory.NewResourceReturns(fakeResource)
			fromVersion = nil
		})

		JustBeforeEach(func() {
			scanErr = scanner.ScanFromVersion(lagertest.NewTestLogger("test"), 57, fromVersion)
		})

		Context("if the lock can be acquired", func() {
			BeforeEach(func() {
				fakeResourceConfigScope.AcquireResourceCheckingLockReturns(fakeLock, true, nil)
				fakeResourceConfigScope.UpdateLastCheckStartTimeReturns(true, nil)
				fakeResourceConfigVersion := new(dbfakes.FakeResourceConfigVersion)
				fakeResourceConfigVersion.IDReturns(1)
				fakeResourceConfigVersion.VersionReturns(db.Version{"custom": "version"})

				fakeResourceConfigScope.LatestVersionReturns(fakeResourceConfigVersion, true, nil)
			})

			Context("when fromVersion is nil", func() {
				It("checks from the current version", func() {
					_, _, version := fakeResourceFactory.NewResourceArgsForCall(0)
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
					_, _, version := fakeResourceFactory.NewResourceArgsForCall(0)
					Expect(version).To(Equal(atc.Version{"version": "1"}))
				})

				Context("when the check returns only the latest version", func() {
					BeforeEach(func() {
						fakeResource.CheckReturns([]atc.Version{fromVersion}, nil)
					})

					It("saves it", func() {
						Expect(fakeResourceConfigScope.SaveVersionsCallCount()).To(Equal(1))
						versions := fakeResourceConfigScope.SaveVersionsArgsForCall(0)
						Expect(versions[0]).To(Equal(fromVersion))
					})
				})
			})

			Context("when checking fails with ErrResourceScriptFailed", func() {
				scriptFail := runtime.ErrResourceScriptFailed{}

				BeforeEach(func() {
					fakeResource.CheckReturns(nil, scriptFail)
				})

				It("returns the error", func() {
					Expect(scanErr).To(Equal(scriptFail))
				})
			})

			Context("when the resource is not in the database", func() {
				BeforeEach(func() {
					fakeDBPipeline.ResourceTypeByIDReturns(nil, false, nil)
				})

				It("returns an error", func() {
					Expect(scanErr).To(HaveOccurred())
					Expect(scanErr.Error()).To(ContainSubstring("resource type not found: 57"))
				})
			})
		})
	})
})
