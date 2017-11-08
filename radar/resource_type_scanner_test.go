package radar_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/db/lock/lockfakes"
	. "github.com/concourse/atc/radar"
	"github.com/concourse/atc/worker"

	rfakes "github.com/concourse/atc/resource/resourcefakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceTypeScanner", func() {
	var (
		epoch time.Time

		fakeResourceFactory                   *rfakes.FakeResourceFactory
		fakeResourceConfigCheckSessionFactory *dbfakes.FakeResourceConfigCheckSessionFactory
		fakeResourceConfigCheckSession        *dbfakes.FakeResourceConfigCheckSession
		fakeDBPipeline                        *dbfakes.FakePipeline
		fakeClock                             *fakeclock.FakeClock
		interval                              time.Duration
		variables                             creds.Variables

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
				Type:   "docker-image",
				Source: atc.Source{"custom": "((source-params))"},
			},
			Version: atc.Version{"custom": "version"},
		}

		fakeResourceFactory = new(rfakes.FakeResourceFactory)
		fakeResourceConfigCheckSessionFactory = new(dbfakes.FakeResourceConfigCheckSessionFactory)
		fakeResourceConfigCheckSession = new(dbfakes.FakeResourceConfigCheckSession)
		fakeResourceType = new(dbfakes.FakeResourceType)
		fakeDBPipeline = new(dbfakes.FakePipeline)
		fakeClock = fakeclock.NewFakeClock(epoch)

		fakeResourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSessionReturns(fakeResourceConfigCheckSession, nil)

		fakeResourceConfigCheckSession.ResourceConfigReturns(&db.UsedResourceConfig{ID: 123})

		fakeResourceType.IDReturns(39)
		fakeResourceType.NameReturns("some-custom-resource")
		fakeResourceType.TypeReturns("docker-image")
		fakeResourceType.SourceReturns(atc.Source{"custom": "((source-params))"})
		fakeResourceType.VersionReturns(atc.Version{"custom": "version"})
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
			fakeResourceConfigCheckSessionFactory,
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
				fakeDBPipeline.AcquireResourceTypeCheckingLockWithIntervalCheckReturns(nil, false, nil)
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
				fakeDBPipeline.AcquireResourceTypeCheckingLockWithIntervalCheckReturns(fakeLock, true, nil)
			})

			It("checks immediately", func() {
				Expect(fakeResource.CheckCallCount()).To(Equal(1))
			})

			It("constructs the resource of the correct type", func() {
				Expect(fakeResourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSessionCallCount()).To(Equal(1))
				_, resourceType, resourceSource, resourceTypes, _ := fakeResourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSessionArgsForCall(0)
				Expect(resourceType).To(Equal("docker-image"))
				Expect(resourceSource).To(Equal(atc.Source{"custom": "some-secret-sauce"}))
				Expect(resourceTypes).To(Equal(creds.VersionedResourceTypes{}))

				Expect(fakeResourceType.SetResourceConfigCallCount()).To(Equal(1))
				resourceConfigID := fakeResourceType.SetResourceConfigArgsForCall(0)
				Expect(resourceConfigID).To(Equal(123))

				Expect(fakeResourceFactory.NewResourceCallCount()).To(Equal(1))
				_, _, owner, metadata, resourceSpec, resourceTypes, _ := fakeResourceFactory.NewResourceArgsForCall(0)
				Expect(owner).To(Equal(db.NewResourceConfigCheckSessionContainerOwner(fakeResourceConfigCheckSession, teamID)))
				Expect(metadata).To(Equal(db.ContainerMetadata{
					Type: db.ContainerTypeCheck,
				}))
				Expect(resourceSpec).To(Equal(worker.ContainerSpec{
					ImageSpec: worker.ImageSpec{
						ResourceType: "docker-image",
					},
					Tags:   []string{},
					TeamID: 123,
				}))
				Expect(resourceTypes).To(Equal(creds.VersionedResourceTypes{}))
			})

			Context("when the resource type overrides a base resource type", func() {
				BeforeEach(func() {
					otherResourceType := fakeResourceType

					fakeResourceType = new(dbfakes.FakeResourceType)
					fakeResourceType.IDReturns(40)
					fakeResourceType.NameReturns("docker-image")
					fakeResourceType.TypeReturns("docker-image")
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
					Expect(fakeResourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSessionCallCount()).To(Equal(1))
					_, resourceType, resourceSource, resourceTypes, _ := fakeResourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSessionArgsForCall(0)
					Expect(resourceType).To(Equal("docker-image"))
					Expect(resourceSource).To(Equal(atc.Source{"custom": "some-secret-sauce"}))
					Expect(resourceTypes).To(Equal(creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
						versionedResourceType,
					})))

					Expect(fakeResourceType.SetResourceConfigCallCount()).To(Equal(1))
					resourceConfigID := fakeResourceType.SetResourceConfigArgsForCall(0)
					Expect(resourceConfigID).To(Equal(123))

					Expect(fakeResourceFactory.NewResourceCallCount()).To(Equal(1))
					_, _, owner, metadata, resourceSpec, resourceTypes, _ := fakeResourceFactory.NewResourceArgsForCall(0)
					Expect(owner).To(Equal(db.NewResourceConfigCheckSessionContainerOwner(fakeResourceConfigCheckSession, teamID)))
					Expect(metadata).To(Equal(db.ContainerMetadata{
						Type: db.ContainerTypeCheck,
					}))
					Expect(resourceSpec).To(Equal(worker.ContainerSpec{
						ImageSpec: worker.ImageSpec{
							ResourceType: "docker-image",
						},
						Tags:   []string{},
						TeamID: 123,
					}))
					Expect(resourceTypes).To(Equal(creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
						versionedResourceType,
					})))
				})
			})

			It("grabs a periodic resource checking lock before checking, breaks lock after done", func() {
				Expect(fakeDBPipeline.AcquireResourceTypeCheckingLockWithIntervalCheckCallCount()).To(Equal(1))

				_, resourceTypeName, resourceConfig, leaseInterval, immediate := fakeDBPipeline.AcquireResourceTypeCheckingLockWithIntervalCheckArgsForCall(0)
				Expect(resourceTypeName).To(Equal(fakeResourceType.Name()))
				Expect(leaseInterval).To(Equal(interval))
				Expect(resourceConfig).To(Equal(fakeResourceConfigCheckSession.ResourceConfig()))
				Expect(immediate).To(BeFalse())

				Eventually(fakeLock.ReleaseCallCount()).Should(Equal(1))
			})

			Context("when there is no current version", func() {
				BeforeEach(func() {
					fakeResourceType.VersionReturns(nil)
				})

				It("checks from nil", func() {
					_, version := fakeResource.CheckArgsForCall(0)
					Expect(version).To(BeNil())
				})
			})

			Context("when there is a current version", func() {
				BeforeEach(func() {
					fakeResourceType.VersionReturns(atc.Version{"version": "42"})
				})

				It("checks with it", func() {
					Expect(fakeResource.CheckCallCount()).To(Equal(1))
					_, version := fakeResource.CheckArgsForCall(0)
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
					fakeResource.CheckStub = func(source atc.Source, from atc.Version) ([]atc.Version, error) {
						defer GinkgoRecover()

						Expect(source).To(Equal(atc.Source{"custom": "some-secret-sauce"}))

						checkedFrom <- from
						result := checkResults[check]
						check++

						return result, nil
					}
				})

				It("saves the latest resource type version", func() {
					Eventually(fakeResourceType.SaveVersionCallCount).Should(Equal(1))

					version := fakeResourceType.SaveVersionArgsForCall(0)
					Expect(version).To(Equal(atc.Version{"version": "3"}))
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
				fakeDBPipeline.AcquireResourceTypeCheckingLockWithIntervalCheckReturns(fakeLock, true, nil)
			})

			It("checks immediately", func() {
				Expect(fakeResource.CheckCallCount()).To(Equal(1))
			})

			It("constructs the resource of the correct type", func() {
				Expect(fakeResourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSessionCallCount()).To(Equal(1))
				_, resourceType, resourceSource, resourceTypes, _ := fakeResourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSessionArgsForCall(0)
				Expect(resourceType).To(Equal("docker-image"))
				Expect(resourceSource).To(Equal(atc.Source{"custom": "some-secret-sauce"}))
				Expect(resourceTypes).To(Equal(creds.VersionedResourceTypes{}))

				Expect(fakeResourceType.SetResourceConfigCallCount()).To(Equal(1))
				resourceConfigID := fakeResourceType.SetResourceConfigArgsForCall(0)
				Expect(resourceConfigID).To(Equal(123))

				Expect(fakeResourceFactory.NewResourceCallCount()).To(Equal(1))
				_, _, owner, metadata, resourceSpec, resourceTypes, _ := fakeResourceFactory.NewResourceArgsForCall(0)
				Expect(owner).To(Equal(db.NewResourceConfigCheckSessionContainerOwner(fakeResourceConfigCheckSession, teamID)))
				Expect(metadata).To(Equal(db.ContainerMetadata{
					Type: db.ContainerTypeCheck,
				}))
				Expect(resourceSpec).To(Equal(worker.ContainerSpec{
					ImageSpec: worker.ImageSpec{
						ResourceType: "docker-image",
					},
					Tags:   []string{},
					TeamID: 123,
				}))
				Expect(resourceTypes).To(Equal(creds.VersionedResourceTypes{}))
			})

			Context("when the resource type depends on another custom type", func() {
				var otherResourceType *dbfakes.FakeResourceType

				BeforeEach(func() {
					otherResourceType = new(dbfakes.FakeResourceType)
					otherResourceType.IDReturns(39)
					otherResourceType.NameReturns("custom-resource-parent")
					otherResourceType.TypeReturns("docker-image")

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
					})
				})

				Context("when the custom type has a version already", func() {
					BeforeEach(func() {
						otherResourceType.VersionReturns(atc.Version{"custom": "image-version"})
					})

					It("does not check for versions of the parent resource type", func() {
						Expect(fakeDBPipeline.ResourceTypeCallCount()).To(Equal(1))
						Expect(fakeResourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSessionCallCount()).To(Equal(1))
					})
				})
			})

			Context("when the resource type overrides a base resource type", func() {
				BeforeEach(func() {
					otherResourceType := fakeResourceType

					fakeResourceType = new(dbfakes.FakeResourceType)
					fakeResourceType.IDReturns(40)
					fakeResourceType.NameReturns("docker-image")
					fakeResourceType.TypeReturns("docker-image")
					fakeResourceType.SourceReturns(atc.Source{"custom": "((source-params))"})

					fakeDBPipeline.ResourceTypesReturns([]db.ResourceType{
						fakeResourceType,
						otherResourceType,
					}, nil)
					fakeDBPipeline.ResourceTypeReturns(fakeResourceType, true, nil)
					fakeResourceType.SetResourceConfigReturns(nil)
				})

				It("constructs the resource of the correct type", func() {
					Expect(fakeResourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSessionCallCount()).To(Equal(1))
					_, resourceType, resourceSource, resourceTypes, _ := fakeResourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSessionArgsForCall(0)
					Expect(resourceType).To(Equal("docker-image"))
					Expect(resourceSource).To(Equal(atc.Source{"custom": "some-secret-sauce"}))
					Expect(resourceTypes).To(Equal(creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
						versionedResourceType,
					})))

					Expect(fakeResourceType.SetResourceConfigCallCount()).To(Equal(1))
					resourceConfigID := fakeResourceType.SetResourceConfigArgsForCall(0)
					Expect(resourceConfigID).To(Equal(123))

					Expect(fakeResourceFactory.NewResourceCallCount()).To(Equal(1))
					_, _, owner, metadata, resourceSpec, resourceTypes, _ := fakeResourceFactory.NewResourceArgsForCall(0)
					Expect(owner).To(Equal(db.NewResourceConfigCheckSessionContainerOwner(fakeResourceConfigCheckSession, teamID)))
					Expect(metadata).To(Equal(db.ContainerMetadata{
						Type: db.ContainerTypeCheck,
					}))
					Expect(resourceSpec).To(Equal(worker.ContainerSpec{
						ImageSpec: worker.ImageSpec{
							ResourceType: "docker-image",
						},
						Tags:   []string{},
						TeamID: 123,
					}))
					Expect(resourceTypes).To(Equal(creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
						versionedResourceType,
					})))
				})
			})

			It("grabs an immediate resource checking lock before checking, breaks lock after done", func() {
				Expect(fakeDBPipeline.AcquireResourceTypeCheckingLockWithIntervalCheckCallCount()).To(Equal(1))

				_, resourceTypeName, resourceConfig, leaseInterval, immediate := fakeDBPipeline.AcquireResourceTypeCheckingLockWithIntervalCheckArgsForCall(0)
				Expect(resourceTypeName).To(Equal(fakeResourceType.Name()))
				Expect(leaseInterval).To(Equal(interval))
				Expect(resourceConfig).To(Equal(fakeResourceConfigCheckSession.ResourceConfig()))
				Expect(immediate).To(BeTrue())

				Eventually(fakeLock.ReleaseCallCount()).Should(Equal(1))
			})

			Context("when there is no current version", func() {
				BeforeEach(func() {
					fakeResourceType.VersionReturns(nil)
				})

				It("checks from nil", func() {
					_, version := fakeResource.CheckArgsForCall(0)
					Expect(version).To(BeNil())
				})
			})

			Context("when there is a current version", func() {
				BeforeEach(func() {
					fakeResourceType.VersionReturns(atc.Version{"version": "42"})
				})

				It("checks with it", func() {
					Expect(fakeResource.CheckCallCount()).To(Equal(1))
					_, version := fakeResource.CheckArgsForCall(0)
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
					fakeResource.CheckStub = func(source atc.Source, from atc.Version) ([]atc.Version, error) {
						defer GinkgoRecover()

						Expect(source).To(Equal(atc.Source{"custom": "some-secret-sauce"}))

						checkedFrom <- from
						result := checkResults[check]
						check++

						return result, nil
					}
				})

				It("saves the latest resource type version", func() {
					Eventually(fakeResourceType.SaveVersionCallCount).Should(Equal(1))

					version := fakeResourceType.SaveVersionArgsForCall(0)
					Expect(version).To(Equal(atc.Version{"version": "3"}))
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

					fakeDBPipeline.AcquireResourceTypeCheckingLockWithIntervalCheckStub = func(logger lager.Logger, resourceName string, resourceConfig *db.UsedResourceConfig, interval time.Duration, immediate bool) (lock.Lock, bool, error) {
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
					Expect(fakeDBPipeline.AcquireResourceTypeCheckingLockWithIntervalCheckCallCount()).To(Equal(3))

					_, resourceTypeName, resourceConfig, leaseInterval, immediate := fakeDBPipeline.AcquireResourceTypeCheckingLockWithIntervalCheckArgsForCall(0)
					Expect(resourceTypeName).To(Equal(fakeResourceType.Name()))
					Expect(leaseInterval).To(Equal(interval))
					Expect(immediate).To(BeTrue())
					Expect(resourceConfig).To(Equal(fakeResourceConfigCheckSession.ResourceConfig()))

					_, resourceTypeName, resourceConfig, leaseInterval, immediate = fakeDBPipeline.AcquireResourceTypeCheckingLockWithIntervalCheckArgsForCall(1)
					Expect(resourceTypeName).To(Equal(fakeResourceType.Name()))
					Expect(leaseInterval).To(Equal(interval))
					Expect(immediate).To(BeTrue())
					Expect(resourceConfig).To(Equal(fakeResourceConfigCheckSession.ResourceConfig()))

					_, resourceTypeName, resourceConfig, leaseInterval, immediate = fakeDBPipeline.AcquireResourceTypeCheckingLockWithIntervalCheckArgsForCall(2)
					Expect(resourceTypeName).To(Equal(fakeResourceType.Name()))
					Expect(leaseInterval).To(Equal(interval))
					Expect(immediate).To(BeTrue())
					Expect(resourceConfig).To(Equal(fakeResourceConfigCheckSession.ResourceConfig()))

					Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
				})
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
})
