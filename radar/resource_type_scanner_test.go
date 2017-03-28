package radar_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/dbng/dbngfakes"
	. "github.com/concourse/atc/radar"
	"github.com/concourse/atc/radar/radarfakes"
	"github.com/concourse/atc/worker"

	rfakes "github.com/concourse/atc/resource/resourcefakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceTypeScanner", func() {
	var (
		fakeResourceFactory *rfakes.FakeResourceFactory
		fakeRadarDB         *radarfakes.FakeRadarDB
		fakeDBPipeline      *dbngfakes.FakePipeline
		interval            time.Duration

		fakeResourceType      *dbngfakes.FakeResourceType
		versionedResourceType atc.VersionedResourceType

		scanner Scanner

		savedResourceType db.SavedResourceType

		fakeLock *lockfakes.FakeLock
		teamID   = 123
	)

	BeforeEach(func() {
		fakeResourceFactory = new(rfakes.FakeResourceFactory)
		fakeRadarDB = new(radarfakes.FakeRadarDB)
		interval = 1 * time.Minute

		fakeDBPipeline = new(dbngfakes.FakePipeline)

		scanner = NewResourceTypeScanner(
			fakeResourceFactory,
			interval,
			fakeRadarDB,
			fakeDBPipeline,
			"https://www.example.com",
		)

		fakeRadarDB.ScopedNameStub = func(thing string) string {
			return "pipeline:" + thing
		}

		fakeRadarDB.ReloadReturns(true, nil)

		savedResourceType = db.SavedResourceType{
			ID:   39,
			Name: "some-resource-type",
			Type: "docker-image",
			Config: atc.ResourceType{
				Name:   "some-resource-type",
				Type:   "docker-image",
				Source: atc.Source{"custom": "source"},
			},
		}

		fakeLock = &lockfakes.FakeLock{}

		fakeRadarDB.GetResourceTypeReturns(savedResourceType, true, nil)

		fakeDBPipeline.IDReturns(42)
		fakeDBPipeline.TeamIDReturns(teamID)

		fakeResourceType = new(dbngfakes.FakeResourceType)
		fakeResourceType.IDReturns(1)
		fakeResourceType.NameReturns("some-custom-resource")
		fakeResourceType.TypeReturns("docker-image")
		fakeResourceType.SourceReturns(atc.Source{"custom": "source"})
		fakeResourceType.VersionReturns(atc.Version{"custom": "version"})
		fakeDBPipeline.ResourceTypesReturns([]dbng.ResourceType{fakeResourceType}, nil)

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
			fakeResourceFactory.NewCheckResourceReturns(fakeResource, nil)
		})

		JustBeforeEach(func() {
			actualInterval, runErr = scanner.Run(lagertest.NewTestLogger("test"), "some-resource-type")
		})

		Context("when the lock cannot be acquired", func() {
			BeforeEach(func() {
				fakeRadarDB.AcquireResourceTypeCheckingLockReturns(nil, false, nil)
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
				fakeRadarDB.AcquireResourceTypeCheckingLockReturns(fakeLock, true, nil)
			})

			It("checks immediately", func() {
				Expect(fakeResource.CheckCallCount()).To(Equal(1))
			})

			It("constructs the resource of the correct type", func() {
				Expect(fakeResource.CheckCallCount()).To(Equal(1))
				Expect(fakeResourceFactory.NewCheckResourceCallCount()).To(Equal(1))
				_, user, metadata, resourceSpec, customTypes, _, resourceConfig := fakeResourceFactory.NewCheckResourceArgsForCall(0)
				Expect(user).To(Equal(dbng.ForResourceType{ResourceTypeID: 39}))
				Expect(metadata).To(Equal(dbng.ContainerMetadata{
					Type:           dbng.ContainerTypeCheck,
					PipelineID:     42,
					ResourceTypeID: 39,
				}))
				Expect(customTypes).To(Equal(atc.VersionedResourceTypes{versionedResourceType}))
				Expect(resourceSpec).To(Equal(worker.ContainerSpec{
					ImageSpec: worker.ImageSpec{
						ResourceType: "docker-image",
						Privileged:   true,
					},
					Ephemeral: true,
					Tags:      []string{},
					TeamID:    123,
				}))
				Expect(resourceConfig).To(Equal(atc.ResourceConfig{
					Type:   "docker-image",
					Source: atc.Source{"custom": "source"},
				}))
			})

			It("grabs a periodic resource checking lock before checking, breaks lock after done", func() {
				Expect(fakeRadarDB.AcquireResourceTypeCheckingLockCallCount()).To(Equal(1))

				_, resourceType, leaseInterval, immediate := fakeRadarDB.AcquireResourceTypeCheckingLockArgsForCall(0)
				Expect(resourceType.Name).To(Equal("some-resource-type"))
				Expect(leaseInterval).To(Equal(interval))
				Expect(immediate).To(BeFalse())

				Eventually(fakeLock.ReleaseCallCount()).Should(Equal(1))
			})

			Context("when there is no current version", func() {
				It("checks from nil", func() {
					_, version := fakeResource.CheckArgsForCall(0)
					Expect(version).To(BeNil())
				})
			})

			Context("when there is a current version", func() {
				BeforeEach(func() {
					savedResourceType.Version = db.Version{"version": "42"}
					fakeRadarDB.GetResourceTypeReturns(savedResourceType, true, nil)
				})

				It("checks with it", func() {
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
					Eventually(fakeRadarDB.SaveResourceTypeVersionCallCount).Should(Equal(1))

					resourceType, version := fakeRadarDB.SaveResourceTypeVersionArgsForCall(0)
					Expect(resourceType).To(Equal(atc.ResourceType{
						Name:   "some-resource-type",
						Type:   "docker-image",
						Source: atc.Source{"custom": "source"},
					}))

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
		})
	})
})
