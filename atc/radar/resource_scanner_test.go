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
	"github.com/concourse/concourse/atc/radar"
	. "github.com/concourse/concourse/atc/radar"
	rfakes "github.com/concourse/concourse/atc/resource/resourcefakes"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	"github.com/concourse/concourse/vars"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceScanner", func() {
	var (
		epoch      time.Time
		scanLogger lager.Logger

		fakeContainer             *workerfakes.FakeContainer
		fakeWorker                *workerfakes.FakeWorker
		fakePool                  *workerfakes.FakePool
		fakeStrategy              *workerfakes.FakeContainerPlacementStrategy
		fakeResourceFactory       *rfakes.FakeResourceFactory
		fakeResourceConfigFactory *dbfakes.FakeResourceConfigFactory
		fakeDBPipeline            *dbfakes.FakePipeline
		fakeClock                 *fakeclock.FakeClock
		fakeVarSourcePool         *credsfakes.FakeVarSourcePool
		fakeSecrets               *credsfakes.FakeSecrets
		interval                  time.Duration
		variables                 vars.Variables

		fakeResourceType          *dbfakes.FakeResourceType
		interpolatedResourceTypes atc.VersionedResourceTypes

		scanner Scanner

		fakeDBResource          *dbfakes.FakeResource
		fakeDBResourceConfig    *dbfakes.FakeResourceConfig
		fakeResourceConfigScope *dbfakes.FakeResourceConfigScope

		fakeLock *lockfakes.FakeLock
		teamID   = 123
	)

	BeforeEach(func() {
		epoch = time.Unix(123, 456).UTC()
		scanLogger = lagertest.NewTestLogger("test")
		fakeLock = &lockfakes.FakeLock{}
		interval = 1 * time.Minute
		GlobalResourceCheckTimeout = 1 * time.Hour

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
				},
				Version: atc.Version{"custom": "version"},
			},
		}

		fakeVarSourcePool = new(credsfakes.FakeVarSourcePool)
		fakeContainer = new(workerfakes.FakeContainer)
		fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)
		fakePool = new(workerfakes.FakePool)
		fakeWorker = new(workerfakes.FakeWorker)
		fakeResourceFactory = new(rfakes.FakeResourceFactory)
		fakeResourceConfigFactory = new(dbfakes.FakeResourceConfigFactory)
		fakeResourceType = new(dbfakes.FakeResourceType)
		fakeDBResource = new(dbfakes.FakeResource)
		fakeDBPipeline = new(dbfakes.FakePipeline)
		fakeDBResourceConfig = new(dbfakes.FakeResourceConfig)
		fakeDBResourceConfig.IDReturns(123)
		fakeDBResourceConfig.OriginBaseResourceTypeReturns(&db.UsedBaseResourceType{ID: 456})
		fakeResourceConfigScope = new(dbfakes.FakeResourceConfigScope)
		fakeResourceConfigScope.IDReturns(456)
		fakeResourceConfigScope.ResourceConfigReturns(fakeDBResourceConfig)

		fakeDBPipeline.IDReturns(42)
		fakeDBPipeline.NameReturns("some-pipeline")
		fakeDBPipeline.TeamIDReturns(teamID)
		fakeClock = fakeclock.NewFakeClock(epoch)

		fakeDBPipeline.ReloadReturns(true, nil)
		fakeDBPipeline.ResourceTypesReturns([]db.ResourceType{fakeResourceType}, nil)
		fakeDBPipeline.ResourceByIDReturns(fakeDBResource, true, nil)
		fakeDBPipeline.VariablesReturns(variables, nil)

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
		fakeDBResource.SetResourceConfigReturns(fakeResourceConfigScope, nil)

		scanner = NewResourceScanner(
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
			actualInterval, runErr = scanner.Run(scanLogger, 39)
		})

		Context("when the lock cannot be acquired", func() {
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

		Context("when the lock can be acquired", func() {
			BeforeEach(func() {
				fakeResourceConfigScope.AcquireResourceCheckingLockReturns(fakeLock, true, nil)
			})

			Context("when the last checked is not able to be updated", func() {
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

			Context("when the last checked fails to update", func() {
				BeforeEach(func() {
					fakeResourceConfigScope.UpdateLastCheckStartTimeReturns(false, errors.New("woops"))
				})

				It("does not check", func() {
					Expect(fakeResource.CheckCallCount()).To(Equal(0))
				})

				It("returns the configured interval", func() {
					Expect(runErr).To(Equal(errors.New("woops")))
					Expect(actualInterval).To(Equal(interval))
				})
			})

			Context("when the last checked is updated", func() {
				BeforeEach(func() {
					fakeResourceConfigScope.UpdateLastCheckStartTimeReturns(true, nil)
				})

				It("checks immediately", func() {
					Expect(fakeResource.CheckCallCount()).To(Equal(1))
				})

				It("constructs the resource of the correct type", func() {
					Expect(fakeDBResource.SetResourceConfigCallCount()).To(Equal(1))
					resourceSource, resourceTypes := fakeDBResource.SetResourceConfigArgsForCall(0)
					Expect(resourceSource).To(Equal(atc.Source{"uri": "some-secret-sauce"}))
					Expect(resourceTypes).To(Equal(interpolatedResourceTypes))

					Expect(fakeDBResource.SetCheckSetupErrorCallCount()).To(Equal(1))
					err := fakeDBResource.SetCheckSetupErrorArgsForCall(0)
					Expect(err).To(BeNil())

					_, _, owner, containerSpec, workerSpec, _ := fakePool.FindOrChooseWorkerForContainerArgsForCall(0)
					Expect(owner).To(Equal(db.NewResourceConfigCheckSessionContainerOwner(123, 456, radar.ContainerExpiries)))
					Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
						ResourceType: "git",
					}))
					Expect(containerSpec.Tags).To(Equal([]string{"some-tag"}))
					Expect(containerSpec.TeamID).To(Equal(123))
					Expect(containerSpec.Env).To(Equal([]string{
						"ATC_EXTERNAL_URL=https://www.example.com",
						"RESOURCE_PIPELINE_NAME=some-pipeline",
						"RESOURCE_NAME=some-resource",
					}))
					Expect(workerSpec).To(Equal(worker.WorkerSpec{
						ResourceType:  "git",
						Tags:          atc.Tags{"some-tag"},
						ResourceTypes: interpolatedResourceTypes,
						TeamID:        123,
					}))

					var metadata db.ContainerMetadata
					Expect(fakeWorker.FindOrCreateContainerCallCount()).To(Equal(1))
					_, _, _, owner, metadata, containerSpec, resourceTypes = fakeWorker.FindOrCreateContainerArgsForCall(0)
					Expect(owner).To(Equal(db.NewResourceConfigCheckSessionContainerOwner(123, 456, radar.ContainerExpiries)))
					Expect(metadata).To(Equal(db.ContainerMetadata{
						Type: db.ContainerTypeCheck,
					}))
					Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
						ResourceType: "git",
					}))
					Expect(containerSpec.Tags).To(Equal([]string{"some-tag"}))
					Expect(containerSpec.TeamID).To(Equal(123))
					Expect(containerSpec.Env).To(Equal([]string{
						"ATC_EXTERNAL_URL=https://www.example.com",
						"RESOURCE_PIPELINE_NAME=some-pipeline",
						"RESOURCE_NAME=some-resource",
					}))
					Expect(resourceTypes).To(Equal(interpolatedResourceTypes))
				})

				Context("when the resource config has a specified check interval", func() {
					BeforeEach(func() {
						fakeDBResource.CheckEveryReturns("10ms")
						fakeDBPipeline.ResourceByIDReturns(fakeDBResource, true, nil)
					})

					It("leases for the configured interval", func() {
						Expect(fakeResourceConfigScope.AcquireResourceCheckingLockCallCount()).To(Equal(1))
						Expect(fakeResourceConfigScope.UpdateLastCheckStartTimeCallCount()).To(Equal(1))

						leaseInterval, immediate := fakeResourceConfigScope.UpdateLastCheckStartTimeArgsForCall(0)
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
							fakeDBPipeline.ResourceByIDReturns(fakeDBResource, true, nil)
						})

						It("sets the check error", func() {
							Expect(fakeDBResource.SetCheckSetupErrorCallCount()).To(Equal(1))

							resourceErr := fakeDBResource.SetCheckSetupErrorArgsForCall(0)
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

					Eventually(fakeLock.ReleaseCallCount).Should(Equal(1))
				})

				Context("when the resource uses a custom type", func() {
					BeforeEach(func() {
						fakeDBResource.TypeReturns("some-custom-resource")
					})

					Context("and the custom type has a version", func() {
						It("doesn't check for check error of custom type", func() {
							Expect(fakeResourceType.CheckErrorCallCount()).To(Equal(0))
						})
					})

					Context("and the custom type does not have a version", func() {
						BeforeEach(func() {
							results := make(chan bool, 4)
							results <- false
							results <- false
							results <- true
							results <- true
							close(results)

							fakeResourceType.VersionStub = func() atc.Version {
								if <-results {
									return atc.Version{"version": "1"}
								} else {
									// allow the sleep to continue
									go fakeClock.WaitForWatcherAndIncrement(10 * time.Second)
									return nil
								}
							}
						})

						Context("when the custom type has a check error", func() {
							BeforeEach(func() {
								fakeResourceType.CheckErrorReturns(errors.New("oops"))
							})

							It("sets the resource check error to the custom type's check error and does not run a check", func() {
								Expect(fakeDBResource.SetCheckSetupErrorCallCount()).To(Equal(1))
								err := fakeDBResource.SetCheckSetupErrorArgsForCall(0)
								Expect(err).To(Equal(errors.New("oops")))

								Expect(fakeResource.CheckCallCount()).To(Equal(0))
							})
						})

						Context("when the custom type has a nil check error", func() {
							Context("when the resource type sucessfully reloads", func() {
								BeforeEach(func() {
									fakeResourceType.ReloadReturns(true, nil)
								})

								It("retries every second until version is not nil", func() {
									Expect(fakeResourceType.VersionCallCount()).To(Equal(4))
								})
							})

							Context("when the resource type fails to reload", func() {
								disaster := errors.New("oops")

								BeforeEach(func() {
									fakeResourceType.ReloadReturns(false, disaster)
								})

								It("returns an error", func() {
									Expect(runErr).To(HaveOccurred())
									Expect(runErr).To(Equal(disaster))
								})
							})

							Context("when the resource type is not found", func() {
								BeforeEach(func() {
									fakeResourceType.ReloadReturns(false, nil)
								})

								It("returns ErrResourceTypeNotFound error", func() {
									Expect(runErr).To(HaveOccurred())
									Expect(runErr).To(Equal(radar.ErrResourceTypeNotFound))
								})
							})
						})
					})
				})

				Context("when there is no current version", func() {
					It("checks from nil", func() {
						_, _, version := fakeResourceFactory.NewResourceArgsForCall(0)
						Expect(version).To(BeNil())
					})
				})

				Context("when there is a current version", func() {
					BeforeEach(func() {
						fakeResourceConfigVersion := new(dbfakes.FakeResourceConfigVersion)
						fakeResourceConfigVersion.IDReturns(1)
						fakeResourceConfigVersion.VersionReturns(db.Version{"version": "1"})

						fakeResourceConfigScope.LatestVersionReturns(fakeResourceConfigVersion, true, nil)
					})

					It("checks from it", func() {
						_, _, version := fakeResourceFactory.NewResourceArgsForCall(0)
						Expect(version).To(Equal(atc.Version{"version": "1"}))
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

					It("saves them all, in order", func() {
						Eventually(fakeResourceConfigScope.SaveVersionsCallCount).Should(Equal(1))

						versions := fakeResourceConfigScope.SaveVersionsArgsForCall(0)
						Expect(versions).To(Equal([]atc.Version{
							{"version": "1"},
							{"version": "2"},
							{"version": "3"},
						}))
					})

					Context("when saving versions fails", func() {
						BeforeEach(func() {
							fakeResourceConfigScope.SaveVersionsReturns(errors.New("failed"))
						})

						It("returns an error", func() {
							Expect(runErr).To(HaveOccurred())
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
					scriptFail := runtime.ErrResourceScriptFailed{}

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
						fakeDBPipeline.ResourceByIDReturns(nil, false, disaster)
					})

					It("returns an error", func() {
						Expect(runErr).To(HaveOccurred())
						Expect(runErr).To(Equal(disaster))
					})
				})

				Context("when the resource is not in the database", func() {
					BeforeEach(func() {
						fakeDBPipeline.ResourceByIDReturns(nil, false, nil)
					})

					It("returns an error", func() {
						Expect(runErr).To(HaveOccurred())
						Expect(runErr.Error()).To(ContainSubstring("resource '39' not found"))
					})
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
			fakeWorker.NameReturns("some-worker")
			fakePool.FindOrChooseWorkerForContainerReturns(fakeWorker, nil)

			fakeContainer.HandleReturns("some-handle")
			fakeWorker.FindOrCreateContainerReturns(fakeContainer, nil)

			fakeResource = new(rfakes.FakeResource)
			fakeResourceFactory.NewResourceReturns(fakeResource)
		})

		JustBeforeEach(func() {
			scanErr = scanner.Scan(lagertest.NewTestLogger("test"), 39)
		})

		Context("if the lock can be acquired and last checked updated", func() {
			BeforeEach(func() {
				fakeResourceConfigScope.AcquireResourceCheckingLockReturns(fakeLock, true, nil)
				fakeResourceConfigScope.UpdateLastCheckStartTimeReturns(true, nil)
			})

			Context("Parent resource has no version and check fails", func() {
				BeforeEach(func() {
					var fakeGitResourceType *dbfakes.FakeResourceType
					fakeGitResourceType = new(dbfakes.FakeResourceType)

					fakeDBPipeline.ResourceTypesReturns([]db.ResourceType{fakeGitResourceType}, nil)

					fakeGitResourceType.IDReturns(5)
					fakeGitResourceType.NameReturns("git")
					fakeGitResourceType.TypeReturns("registry-image")
					fakeGitResourceType.SourceReturns(atc.Source{"custom": "((source-params))"})
					fakeGitResourceType.VersionReturns(nil)
					fakeGitResourceType.CheckErrorReturns(errors.New("oops"))
				})

				It("fails and returns error", func() {
					Expect(scanErr).To(HaveOccurred())
					Expect(scanErr).To(Equal(radar.ErrResourceTypeCheckError))
				})

				It("saves the error to check_error on resource row in db", func() {
					Expect(fakeDBResource.SetCheckSetupErrorCallCount()).To(Equal(1))

					err := fakeDBResource.SetCheckSetupErrorArgsForCall(0)
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(errors.New("oops")))
				})
			})

			Context("Parent resource has a version and but check is failing", func() {
				BeforeEach(func() {
					var fakeGitResourceType *dbfakes.FakeResourceType
					fakeGitResourceType = new(dbfakes.FakeResourceType)

					fakeDBPipeline.ResourceTypesReturns([]db.ResourceType{fakeGitResourceType}, nil)

					fakeGitResourceType.IDReturns(5)
					fakeGitResourceType.NameReturns("git")
					fakeGitResourceType.TypeReturns("registry-image")
					fakeGitResourceType.SourceReturns(atc.Source{"custom": "((source-params))"})
					fakeGitResourceType.VersionReturns(atc.Version{"version": "1"})
					fakeGitResourceType.CheckErrorReturns(errors.New("oops"))
				})

				It("continues to scan", func() {
					Expect(scanErr).NotTo(HaveOccurred())
				})
			})

			It("succeeds", func() {
				Expect(scanErr).NotTo(HaveOccurred())
			})

			It("constructs the resource of the correct type", func() {
				Expect(fakeDBResource.SetResourceConfigCallCount()).To(Equal(1))
				resourceSource, resourceTypes := fakeDBResource.SetResourceConfigArgsForCall(0)
				Expect(resourceSource).To(Equal(atc.Source{"uri": "some-secret-sauce"}))
				Expect(resourceTypes).To(Equal(interpolatedResourceTypes))

				Expect(fakeDBResource.SetCheckSetupErrorCallCount()).To(Equal(1))
				err := fakeDBResource.SetCheckSetupErrorArgsForCall(0)
				Expect(err).To(BeNil())

				_, _, owner, containerSpec, workerSpec, _ := fakePool.FindOrChooseWorkerForContainerArgsForCall(0)
				Expect(owner).To(Equal(db.NewResourceConfigCheckSessionContainerOwner(123, 456, radar.ContainerExpiries)))
				Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
					ResourceType: "git",
				}))
				Expect(containerSpec.Tags).To(Equal([]string{"some-tag"}))
				Expect(containerSpec.TeamID).To(Equal(123))
				Expect(containerSpec.Env).To(Equal([]string{
					"ATC_EXTERNAL_URL=https://www.example.com",
					"RESOURCE_PIPELINE_NAME=some-pipeline",
					"RESOURCE_NAME=some-resource",
				}))
				Expect(workerSpec).To(Equal(worker.WorkerSpec{
					ResourceType:  "git",
					Tags:          atc.Tags{"some-tag"},
					ResourceTypes: interpolatedResourceTypes,
					TeamID:        123,
				}))

				var metadata db.ContainerMetadata
				_, _, _, owner, metadata, containerSpec, resourceTypes = fakeWorker.FindOrCreateContainerArgsForCall(0)
				Expect(owner).To(Equal(db.NewResourceConfigCheckSessionContainerOwner(123, 456, radar.ContainerExpiries)))
				Expect(metadata).To(Equal(db.ContainerMetadata{
					Type: db.ContainerTypeCheck,
				}))
				Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
					ResourceType: "git",
				}))
				Expect(containerSpec.Tags).To(Equal([]string{"some-tag"}))
				Expect(containerSpec.TeamID).To(Equal(123))
				Expect(containerSpec.Env).To(Equal([]string{
					"ATC_EXTERNAL_URL=https://www.example.com",
					"RESOURCE_PIPELINE_NAME=some-pipeline",
					"RESOURCE_NAME=some-resource",
				}))
				Expect(resourceTypes).To(Equal(interpolatedResourceTypes))
			})

			It("grabs an immediate resource checking lock before checking, breaks lock after done", func() {
				Expect(fakeResourceConfigScope.AcquireResourceCheckingLockCallCount()).To(Equal(1))
				Expect(fakeResourceConfigScope.UpdateLastCheckStartTimeCallCount()).To(Equal(1))

				leaseInterval, immediate := fakeResourceConfigScope.UpdateLastCheckStartTimeArgsForCall(0)
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
					Expect(fakeDBResource.SetCheckSetupErrorCallCount()).To(Equal(1))

					resourceErr := fakeDBResource.SetCheckSetupErrorArgsForCall(0)
					Expect(resourceErr).To(MatchError("catastrophe"))
				})
			})

			Context("when creating the container fails", func() {
				BeforeEach(func() {
					fakeWorker.FindOrCreateContainerReturns(nil, errors.New("catastrophe"))
				})

				It("sets the check error and returns the error", func() {
					Expect(scanErr).To(HaveOccurred())
					Expect(fakeResourceConfigScope.SetCheckErrorCallCount()).To(Equal(1))

					resourceErr := fakeResourceConfigScope.SetCheckErrorArgsForCall(0)
					Expect(resourceErr).To(MatchError("catastrophe"))
				})
			})

			Context("when find or choosing the worker fails", func() {
				BeforeEach(func() {
					fakePool.FindOrChooseWorkerForContainerReturns(nil, errors.New("catastrophe"))
				})

				It("sets the check error and returns the error", func() {
					Expect(scanErr).To(HaveOccurred())
					Expect(fakeResourceConfigScope.SetCheckErrorCallCount()).To(Equal(1))

					resourceErr := fakeResourceConfigScope.SetCheckErrorArgsForCall(0)
					Expect(resourceErr).To(MatchError("catastrophe"))
				})
			})

			Context("when the resource config has a specified check interval", func() {
				BeforeEach(func() {
					fakeDBResource.CheckEveryReturns("10ms")
					fakeDBPipeline.ResourceByIDReturns(fakeDBResource, true, nil)
				})

				It("leases for the configured interval", func() {
					Expect(fakeResourceConfigScope.AcquireResourceCheckingLockCallCount()).To(Equal(1))
					Expect(fakeResourceConfigScope.UpdateLastCheckStartTimeCallCount()).To(Equal(1))

					leaseInterval, immediate := fakeResourceConfigScope.UpdateLastCheckStartTimeArgsForCall(0)
					Expect(leaseInterval).To(Equal(10 * time.Millisecond))
					Expect(immediate).To(BeTrue())

					Eventually(fakeLock.ReleaseCallCount).Should(Equal(1))
				})

				Context("when the interval cannot be parsed", func() {
					BeforeEach(func() {
						fakeDBResource.CheckEveryReturns("bad-value")
						fakeDBPipeline.ResourceByIDReturns(fakeDBResource, true, nil)
					})

					It("sets the check error and returns the error", func() {
						Expect(scanErr).To(HaveOccurred())
						Expect(fakeDBResource.SetCheckSetupErrorCallCount()).To(Equal(1))

						resourceErr := fakeDBResource.SetCheckSetupErrorArgsForCall(0)
						Expect(resourceErr).To(MatchError("time: invalid duration bad-value"))
					})
				})
			})

			Context("when the resource has a specified timeout", func() {
				BeforeEach(func() {
					fakeDBResource.CheckTimeoutReturns("10s")
					fakeDBPipeline.ResourceByIDReturns(fakeDBResource, true, nil)
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
						fakeDBPipeline.ResourceByIDReturns(fakeDBResource, true, nil)
					})

					It("fails to parse the timeout and returns the error", func() {
						Expect(scanErr).To(HaveOccurred())
						Expect(fakeDBResource.SetCheckSetupErrorCallCount()).To(Equal(1))

						resourceErr := fakeDBResource.SetCheckSetupErrorArgsForCall(0)
						Expect(resourceErr).To(MatchError("time: invalid duration bad-value"))
					})
				})
			})

			Context("when the resource has a pinned version", func() {
				BeforeEach(func() {
					fakeDBResource.CurrentPinnedVersionReturns(atc.Version{"version": "1"})
				})

				It("tries to find the version in the database", func() {
					Expect(fakeResourceConfigScope.FindVersionCallCount()).To(Equal(1))
					Expect(fakeResourceConfigScope.FindVersionArgsForCall(0)).To(Equal(atc.Version{"version": "1"}))
				})

				Context("when finding the version succeeds", func() {
					BeforeEach(func() {
						fakeResourceConfigVersion := new(dbfakes.FakeResourceConfigVersion)
						fakeResourceConfigVersion.IDReturns(1)
						fakeResourceConfigVersion.VersionReturns(db.Version{"version": "1"})
						fakeResourceConfigScope.FindVersionReturns(fakeResourceConfigVersion, true, nil)
					})

					It("does not check", func() {
						Expect(fakeResource.CheckCallCount()).To(Equal(0))
					})
				})

				Context("when the version is not found", func() {
					BeforeEach(func() {
						fakeResourceConfigScope.FindVersionReturns(nil, false, nil)
					})

					It("checks from the pinned version", func() {
						_, _, version := fakeResourceFactory.NewResourceArgsForCall(0)
						Expect(version).To(Equal(atc.Version{"version": "1"}))
					})
				})

				Context("when finding the version fails", func() {
					BeforeEach(func() {
						fakeResourceConfigScope.FindVersionReturns(nil, false, errors.New("ah"))
					})

					It("sets the check error on the resource config", func() {
						Expect(fakeResourceConfigScope.SetCheckErrorCallCount()).To(Equal(1))

						err := fakeResourceConfigScope.SetCheckErrorArgsForCall(0)
						Expect(err).To(Equal(errors.New("ah")))
					})
				})
			})

			It("clears the resource's check error", func() {
				Expect(fakeResourceConfigScope.SetCheckErrorCallCount()).To(Equal(1))

				err := fakeResourceConfigScope.SetCheckErrorArgsForCall(0)
				Expect(err).To(BeNil())
			})

			It("invokes resourceFactory.NewResource with the correct arguments", func() {
				actualSource, actualParams, actualVersion := fakeResourceFactory.NewResourceArgsForCall(0)
				Expect(actualSource).To(Equal(atc.Source{"uri": "some-secret-sauce"}))
				Expect(actualParams).To(BeNil())
				Expect(actualVersion).To(BeNil())
			})

			Context("when there is no current version", func() {
				BeforeEach(func() {
					fakeResourceConfigScope.LatestVersionReturns(nil, false, nil)
				})

				It("checks from nil", func() {
					_, _, version := fakeResourceFactory.NewResourceArgsForCall(0)
					Expect(version).To(BeNil())
				})
			})

			Context("when getting the current version fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeResourceConfigScope.LatestVersionReturns(nil, false, disaster)
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

					fakeResourceConfigScope.LatestVersionReturns(fakeResourceConfigVersion, true, nil)
				})

				It("checks from it", func() {
					_, _, version := fakeResourceFactory.NewResourceArgsForCall(0)
					Expect(version).To(Equal(atc.Version{"version": "1"}))
				})

				Context("when the check returns only the latest version", func() {
					BeforeEach(func() {
						fakeResource.CheckReturns([]atc.Version{atc.Version(latestVersion)}, nil)
					})

					It("does not save it", func() {
						Expect(fakeResourceConfigScope.SaveVersionsCallCount()).To(Equal(0))
					})
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

				It("saves them all, in order", func() {
					Expect(fakeResourceConfigScope.SaveVersionsCallCount()).To(Equal(1))

					versions := fakeResourceConfigScope.SaveVersionsArgsForCall(0)
					Expect(versions).To(Equal([]atc.Version{
						{"version": "1"},
						{"version": "2"},
						{"version": "3"},
					}))
				})

				It("updates last check finished", func() {
					Expect(fakeResourceConfigScope.UpdateLastCheckEndTimeCallCount()).To(Equal(1))
				})

				Context("when saving fails", func() {
					BeforeEach(func() {
						fakeResourceConfigScope.SaveVersionsReturns(errors.New("some-error"))
					})

					It("does not update last check finished", func() {
						Expect(fakeResourceConfigScope.UpdateLastCheckEndTimeCallCount()).To(BeZero())
					})
				})
			})

			Context("when the check does not return any new versions", func() {
				BeforeEach(func() {
					fakeResource.CheckStub = func(ctx context.Context, processSpec runtime.ProcessSpec, runner runtime.Runner) ([]atc.Version, error) {
						return []atc.Version{}, nil
					}
				})

				It("updates last check finished", func() {
					Expect(fakeResourceConfigScope.UpdateLastCheckEndTimeCallCount()).To(Equal(1))
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
					Expect(fakeResourceConfigScope.SetCheckErrorCallCount()).To(Equal(1))

					err := fakeResourceConfigScope.SetCheckErrorArgsForCall(0)
					Expect(err).To(Equal(disaster))
				})
			})

			Context("when checking fails with ErrResourceScriptFailed", func() {
				scriptFail := runtime.ErrResourceScriptFailed{}

				BeforeEach(func() {
					fakeResource.CheckReturns(nil, scriptFail)
				})

				It("returns no error", func() {
					Expect(scanErr).NotTo(HaveOccurred())
				})

				It("sets the resource's check error", func() {
					Expect(fakeResourceConfigScope.SetCheckErrorCallCount()).To(Equal(1))

					err := fakeResourceConfigScope.SetCheckErrorArgsForCall(0)
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
			fakeWorker.NameReturns("some-worker")
			fakePool.FindOrChooseWorkerForContainerReturns(fakeWorker, nil)

			fakeContainer.HandleReturns("some-handle")
			fakeWorker.FindOrCreateContainerReturns(fakeContainer, nil)

			fakeResource = new(rfakes.FakeResource)
			fakeResourceFactory.NewResourceReturns(fakeResource)

			fromVersion = nil
		})

		JustBeforeEach(func() {
			scanErr = scanner.ScanFromVersion(lagertest.NewTestLogger("test"), 39, fromVersion)
		})

		Context("if the lock can be acquired and last checked updated", func() {
			BeforeEach(func() {
				fakeResourceConfigScope.AcquireResourceCheckingLockReturns(fakeLock, true, nil)
				fakeResourceConfigScope.UpdateLastCheckStartTimeReturns(true, nil)
			})

			Context("when fromVersion is nil", func() {
				It("checks from nil", func() {
					_, _, version := fakeResourceFactory.NewResourceArgsForCall(0)
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
						Expect(versions).To(Equal([]atc.Version{fromVersion}))
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
					fakeDBPipeline.ResourceByIDReturns(nil, false, nil)
				})

				It("returns an error", func() {
					Expect(scanErr).To(HaveOccurred())
					Expect(scanErr.Error()).To(ContainSubstring("resource '39' not found"))
				})
			})
		})
	})
})
