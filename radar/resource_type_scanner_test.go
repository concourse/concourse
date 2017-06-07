package radar_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/db/lock/lockfakes"
	. "github.com/concourse/atc/radar"
	"github.com/concourse/atc/worker"

	rfakes "github.com/concourse/atc/resource/resourcefakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceTypeScanner", func() {
	var (
		fakeResourceFactory       *rfakes.FakeResourceFactory
		fakeResourceConfigFactory *dbfakes.FakeResourceConfigFactory
		fakeResourceConfig        *db.UsedResourceConfig
		fakeDBPipeline            *dbfakes.FakePipeline
		interval                  time.Duration

		fakeResourceType      *dbfakes.FakeResourceType
		versionedResourceType atc.VersionedResourceType

		scanner Scanner

		fakeLock *lockfakes.FakeLock
		teamID   = 123
	)

	BeforeEach(func() {
		fakeResourceFactory = new(rfakes.FakeResourceFactory)
		fakeResourceConfigFactory = new(dbfakes.FakeResourceConfigFactory)
		fakeResourceConfig = &db.UsedResourceConfig{}
		fakeResourceConfigFactory.FindOrCreateResourceConfigReturns(fakeResourceConfig, nil)

		interval = 1 * time.Minute

		fakeDBPipeline = new(dbfakes.FakePipeline)

		scanner = NewResourceTypeScanner(
			fakeResourceFactory,
			fakeResourceConfigFactory,
			interval,
			fakeDBPipeline,
			"https://www.example.com",
		)

		fakeDBPipeline.ReloadReturns(true, nil)

		fakeLock = &lockfakes.FakeLock{}

		fakeDBPipeline.IDReturns(42)
		fakeDBPipeline.NameReturns("some-pipeline")
		fakeDBPipeline.TeamIDReturns(teamID)

		fakeResourceType = new(dbfakes.FakeResourceType)
		fakeResourceType.IDReturns(39)
		fakeResourceType.NameReturns("some-custom-resource")
		fakeResourceType.TypeReturns("docker-image")
		fakeResourceType.SourceReturns(atc.Source{"custom": "source"})
		fakeResourceType.VersionReturns(atc.Version{"custom": "version"})

		fakeDBPipeline.ResourceTypesReturns([]db.ResourceType{fakeResourceType}, nil)
		fakeDBPipeline.ResourceTypeReturns(fakeResourceType, true, nil)

		versionedResourceType = atc.VersionedResourceType{
			ResourceType: atc.ResourceType{
				Name:   "some-custom-resource",
				Type:   "docker-image",
				Source: atc.Source{"custom": "source"},
			},
			Version: atc.Version{"custom": "version"},
		}
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
				Expect(fakeResourceConfigFactory.FindOrCreateResourceConfigCallCount()).To(Equal(1))
				_, user, resourceType, resourceSource, resourceTypes := fakeResourceConfigFactory.FindOrCreateResourceConfigArgsForCall(0)
				Expect(user).To(Equal(db.ForResourceType(39)))
				Expect(resourceType).To(Equal("docker-image"))
				Expect(resourceSource).To(Equal(atc.Source{"custom": "source"}))
				Expect(resourceTypes).To(Equal(atc.VersionedResourceTypes{}))

				Expect(fakeResourceFactory.NewResourceCallCount()).To(Equal(1))
				_, _, user, owner, metadata, resourceSpec, resourceTypes, _ := fakeResourceFactory.NewResourceArgsForCall(0)
				Expect(user).To(Equal(db.ForResourceType(39)))
				Expect(owner).To(Equal(db.NewResourceConfigCheckSessionContainerOwner(fakeResourceConfig)))
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
				Expect(resourceTypes).To(Equal(atc.VersionedResourceTypes{}))
			})

			Context("when the resource type overrides a base resource type", func() {
				BeforeEach(func() {
					otherResourceType := fakeResourceType

					fakeResourceType = new(dbfakes.FakeResourceType)
					fakeResourceType.IDReturns(40)
					fakeResourceType.NameReturns("docker-image")
					fakeResourceType.TypeReturns("docker-image")
					fakeResourceType.SourceReturns(atc.Source{"custom": "image-source"})
					fakeResourceType.VersionReturns(atc.Version{"custom": "image-version"})

					fakeDBPipeline.ResourceTypesReturns([]db.ResourceType{
						fakeResourceType,
						otherResourceType,
					}, nil)
					fakeDBPipeline.ResourceTypeReturns(fakeResourceType, true, nil)
				})

				It("constructs the resource of the correct type", func() {
					Expect(fakeResourceConfigFactory.FindOrCreateResourceConfigCallCount()).To(Equal(1))
					_, user, resourceType, resourceSource, resourceTypes := fakeResourceConfigFactory.FindOrCreateResourceConfigArgsForCall(0)
					Expect(user).To(Equal(db.ForResourceType(40)))
					Expect(resourceType).To(Equal("docker-image"))
					Expect(resourceSource).To(Equal(atc.Source{"custom": "image-source"}))
					Expect(resourceTypes).To(Equal(atc.VersionedResourceTypes{versionedResourceType}))

					Expect(fakeResourceFactory.NewResourceCallCount()).To(Equal(1))
					_, _, user, owner, metadata, resourceSpec, resourceTypes, _ := fakeResourceFactory.NewResourceArgsForCall(0)
					Expect(user).To(Equal(db.ForResourceType(40)))
					Expect(owner).To(Equal(db.NewResourceConfigCheckSessionContainerOwner(fakeResourceConfig)))
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
					Expect(resourceTypes).To(Equal(atc.VersionedResourceTypes{versionedResourceType}))
				})
			})

			It("grabs a periodic resource checking lock before checking, breaks lock after done", func() {
				Expect(fakeDBPipeline.AcquireResourceTypeCheckingLockWithIntervalCheckCallCount()).To(Equal(1))

				_, resourceTypeName, leaseInterval, immediate := fakeDBPipeline.AcquireResourceTypeCheckingLockWithIntervalCheckArgsForCall(0)
				Expect(resourceTypeName).To(Equal(fakeResourceType.Name()))
				Expect(leaseInterval).To(Equal(interval))
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

						Expect(source).To(Equal(atc.Source{"custom": "source"}))

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
})
