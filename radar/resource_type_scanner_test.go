package radar_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/concourse/atc/radar"
	"github.com/concourse/atc/radar/radarfakes"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"

	"github.com/concourse/atc/db/dbfakes"
	rfakes "github.com/concourse/atc/resource/resourcefakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceTypeScanner", func() {
	var (
		epoch time.Time

		fakeTracker *rfakes.FakeTracker
		fakeRadarDB *radarfakes.FakeRadarDB
		interval    time.Duration

		scanner Scanner

		savedResourceType db.SavedResourceType

		fakeLease *dbfakes.FakeLease
		teamID    = 123
	)

	BeforeEach(func() {
		epoch = time.Unix(123, 456).UTC()
		fakeTracker = new(rfakes.FakeTracker)
		fakeRadarDB = new(radarfakes.FakeRadarDB)
		interval = 1 * time.Minute

		fakeRadarDB.GetPipelineIDReturns(42)
		scanner = NewResourceTypeScanner(
			fakeTracker,
			interval,
			fakeRadarDB,
			"https://www.example.com",
		)

		fakeRadarDB.ScopedNameStub = func(thing string) string {
			return "pipeline:" + thing
		}

		fakeRadarDB.GetConfigReturns(atc.Config{
			ResourceTypes: atc.ResourceTypes{
				{
					Name:   "some-resource-type",
					Type:   "docker-image",
					Source: atc.Source{"custom": "source"},
				},
			},
		}, 1, true, nil)

		savedResourceType = db.SavedResourceType{
			ID:   39,
			Name: "some-resource-type",
			Type: "docker-image",
		}
		fakeRadarDB.TeamIDReturns(teamID)

		fakeLease = &dbfakes.FakeLease{}

		fakeRadarDB.GetResourceTypeReturns(savedResourceType, true, nil)
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
			actualInterval, runErr = scanner.Run(lagertest.NewTestLogger("test"), "some-resource-type")
		})

		Context("when the lease cannot be acquired", func() {
			BeforeEach(func() {
				fakeRadarDB.LeaseResourceTypeCheckingReturns(nil, false, nil)
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
				fakeRadarDB.LeaseResourceTypeCheckingReturns(fakeLease, true, nil)
			})

			It("checks immediately", func() {
				Expect(fakeResource.CheckCallCount()).To(Equal(1))
			})

			It("constructs the resource of the correct type", func() {
				Expect(fakeTracker.InitCallCount()).To(Equal(1))
				_, metadata, session, typ, tags, actualTeamID, customTypes, delegate := fakeTracker.InitArgsForCall(0)
				Expect(metadata).To(Equal(resource.EmptyMetadata{}))

				Expect(session).To(Equal(resource.Session{
					ID: worker.Identifier{
						Stage:               db.ContainerStageCheck,
						CheckType:           "docker-image",
						CheckSource:         atc.Source{"custom": "source"},
						ImageResourceType:   "docker-image",
						ImageResourceSource: atc.Source{"custom": "source"},
					},
					Metadata: worker.Metadata{
						Type:                 db.ContainerTypeCheck,
						PipelineID:           42,
						WorkingDirectory:     "",
						EnvironmentVariables: nil,
					},
					Ephemeral: true,
				}))
				Expect(typ).To(Equal(resource.ResourceType("docker-image")))
				Expect(tags).To(BeEmpty()) // This allows the check to run on any worker
				Expect(actualTeamID).To(Equal(teamID))
				Expect(customTypes).To(Equal(atc.ResourceTypes{}))
				Expect(delegate).To(Equal(worker.NoopImageFetchingDelegate{}))
			})

			It("grabs a periodic resource checking lease before checking, breaks lease after done", func() {
				Expect(fakeRadarDB.LeaseResourceTypeCheckingCallCount()).To(Equal(1))

				_, resourceTypeName, leaseInterval, immediate := fakeRadarDB.LeaseResourceTypeCheckingArgsForCall(0)
				Expect(resourceTypeName).To(Equal("some-resource-type"))
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
