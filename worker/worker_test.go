package worker_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/garden/gardenfakes"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db/dbfakes"
	. "github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/workerfakes"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/baggageclaimfakes"
	"github.com/cppforlife/go-semi-semantic/version"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Worker", func() {
	var (
		logger                     *lagertest.TestLogger
		fakeVolumeClient           *wfakes.FakeVolumeClient
		fakeImageFactory           *wfakes.FakeImageFactory
		fakeClock                  *fakeclock.FakeClock
		fakeDBResourceCacheFactory *dbfakes.FakeResourceCacheFactory
		fakeResourceConfigFactory  *dbfakes.FakeResourceConfigFactory
		fakeContainerProvider      *wfakes.FakeContainerProvider
		activeContainers           int
		resourceTypes              []atc.WorkerResourceType
		platform                   string
		tags                       atc.Tags
		teamID                     int
		workerName                 string
		workerStartTime            int64
		workerUptime               uint64
		gardenWorker               Worker
		workerVersion              string
		fakeGardenClient           *gardenfakes.FakeClient
		fakeBaggageClaimClient     *baggageclaimfakes.FakeClient
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakeVolumeClient = new(wfakes.FakeVolumeClient)
		fakeImageFactory = new(wfakes.FakeImageFactory)
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))
		activeContainers = 42
		resourceTypes = []atc.WorkerResourceType{
			{
				Type:    "some-resource",
				Image:   "some-resource-image",
				Version: "some-version",
			},
		}
		platform = "some-platform"
		tags = atc.Tags{"some", "tags"}
		teamID = 17
		workerName = "some-worker"
		workerStartTime = fakeClock.Now().Unix()
		workerUptime = 0
		workerVersion = "1.2.3"

		fakeDBResourceCacheFactory = new(dbfakes.FakeResourceCacheFactory)
		fakeResourceConfigFactory = new(dbfakes.FakeResourceConfigFactory)
		fakeContainerProvider = new(wfakes.FakeContainerProvider)
		fakeGardenClient = new(gardenfakes.FakeClient)
		fakeBaggageClaimClient = new(baggageclaimfakes.FakeClient)
	})

	JustBeforeEach(func() {
		dbWorker := new(dbfakes.FakeWorker)
		dbWorker.ActiveContainersReturns(activeContainers)
		dbWorker.ResourceTypesReturns(resourceTypes)
		dbWorker.PlatformReturns(platform)
		dbWorker.TagsReturns(tags)
		dbWorker.TeamIDReturns(teamID)
		dbWorker.NameReturns(workerName)
		dbWorker.StartTimeReturns(workerStartTime)
		dbWorker.VersionReturns(&workerVersion)

		gardenWorker = NewGardenWorker(
			fakeGardenClient,
			fakeBaggageClaimClient,
			fakeContainerProvider,
			fakeVolumeClient,
			dbWorker,
			fakeClock,
		)

		fakeClock.IncrementBySeconds(workerUptime)
	})

	Describe("IsVersionCompatible", func() {
		It("is compatible when versions are the same", func() {
			requiredVersion := version.MustNewVersionFromString("1.2.3")
			Expect(
				gardenWorker.IsVersionCompatible(logger, &requiredVersion),
			).To(BeTrue())
		})

		It("is not compatible when versions are different in major version", func() {
			requiredVersion := version.MustNewVersionFromString("2.2.3")
			Expect(
				gardenWorker.IsVersionCompatible(logger, &requiredVersion),
			).To(BeFalse())
		})

		It("is compatible when worker minor version is newer", func() {
			requiredVersion := version.MustNewVersionFromString("1.1.3")
			Expect(
				gardenWorker.IsVersionCompatible(logger, &requiredVersion),
			).To(BeTrue())
		})

		It("is not compatible when worker minor version is older", func() {
			requiredVersion := version.MustNewVersionFromString("1.3.3")
			Expect(
				gardenWorker.IsVersionCompatible(logger, &requiredVersion),
			).To(BeFalse())
		})

		Context("when worker version is empty", func() {
			BeforeEach(func() {
				workerVersion = ""
			})

			It("is not compatible", func() {
				requiredVersion := version.MustNewVersionFromString("1.2.3")
				Expect(
					gardenWorker.IsVersionCompatible(logger, &requiredVersion),
				).To(BeFalse())
			})
		})

		Context("when worker version does not have minor version", func() {
			BeforeEach(func() {
				workerVersion = "1"
			})

			It("is compatible when it is the same", func() {
				requiredVersion := version.MustNewVersionFromString("1")
				Expect(
					gardenWorker.IsVersionCompatible(logger, &requiredVersion),
				).To(BeTrue())
			})

			It("is not compatible when it is different", func() {
				requiredVersion := version.MustNewVersionFromString("2")
				Expect(
					gardenWorker.IsVersionCompatible(logger, &requiredVersion),
				).To(BeFalse())
			})

			It("is not compatible when compared version has minor vesion", func() {
				requiredVersion := version.MustNewVersionFromString("1.2")
				Expect(
					gardenWorker.IsVersionCompatible(logger, &requiredVersion),
				).To(BeFalse())
			})
		})

		Context("when required version is nil", func() {
			It("is compatible", func() {
				Expect(
					gardenWorker.IsVersionCompatible(logger, nil),
				).To(BeTrue())
			})
		})
	})

	Describe("EnsureCertsVolumeExists", func() {
		var ensureErr error
		var expectedCertsVolumeName = "certificates"
		JustBeforeEach(func() {
			ensureErr = gardenWorker.EnsureCertsVolumeExists(logger)
		})

		It("looks up the existing volume in baggageclaim", func() {
			Expect(fakeBaggageClaimClient.LookupVolumeCallCount()).To(Equal(1))
			_, certsVolumeName := fakeBaggageClaimClient.LookupVolumeArgsForCall(0)
			Expect(certsVolumeName).To(Equal(expectedCertsVolumeName))
		})

		Context("when looking the volume up fails", func() {
			var lookupErr = errors.New("failure")
			BeforeEach(func() {
				fakeBaggageClaimClient.LookupVolumeReturns(nil, true, lookupErr)
			})
			It("returns the error", func() {
				Expect(ensureErr).To(Equal(lookupErr))
			})
		})

		Context("when the volume already exists", func() {
			BeforeEach(func() {
				fakeBaggageClaimClient.LookupVolumeReturns(nil, true, nil)
			})

			It("does not create a new volume", func() {
				Expect(fakeBaggageClaimClient.CreateVolumeCallCount()).To(Equal(0))
			})
		})

		Context("when the volume does not exist", func() {
			It("uses certs directory from the host", func() {
				expectedStrategy := baggageclaim.ImportStrategy{
					Path:           "/etc/ssl/certs",
					FollowSymlinks: true,
				}
				By("creating the volume")
				Expect(fakeBaggageClaimClient.CreateVolumeCallCount()).To(Equal(1))
				_, certsVolumeName, volumeSpec := fakeBaggageClaimClient.CreateVolumeArgsForCall(0)

				Expect(certsVolumeName).To(Equal(expectedCertsVolumeName))
				Expect(volumeSpec.Strategy).To(Equal(expectedStrategy))
			})

			Context("when looking the volume up fails", func() {
				var createErr = errors.New("failure")
				BeforeEach(func() {
					fakeBaggageClaimClient.CreateVolumeReturns(nil, createErr)
				})
				It("returns the error", func() {
					Expect(ensureErr).To(Equal(createErr))
				})
			})

		})
	})
	Describe("FindCreatedContainerByHandle", func() {
		var (
			handle            string
			foundContainer    Container
			existingContainer *wfakes.FakeContainer
			found             bool
			checkErr          error
		)

		BeforeEach(func() {
			handle = "we98lsv"
			existingContainer = new(wfakes.FakeContainer)
			fakeContainerProvider.FindCreatedContainerByHandleReturns(existingContainer, true, nil)
		})

		JustBeforeEach(func() {
			foundContainer, found, checkErr = gardenWorker.FindContainerByHandle(logger, 42, handle)
		})

		It("calls the container provider", func() {
			Expect(fakeContainerProvider.FindCreatedContainerByHandleCallCount()).To(Equal(1))

			Expect(foundContainer).To(Equal(existingContainer))
			Expect(checkErr).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

	})

	Describe("Satisfying", func() {
		var (
			spec WorkerSpec

			satisfyingWorker Worker
			satisfyingErr    error

			customTypes creds.VersionedResourceTypes
		)

		BeforeEach(func() {
			spec = WorkerSpec{
				Tags:   []string{"some", "tags"},
				TeamID: teamID,
			}

			variables := template.StaticVariables{}

			customTypes = creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
				{
					ResourceType: atc.ResourceType{
						Name:   "custom-type-b",
						Type:   "custom-type-a",
						Source: atc.Source{"some": "source"},
					},
					Version: atc.Version{"some": "version"},
				},
				{
					ResourceType: atc.ResourceType{
						Name:   "custom-type-a",
						Type:   "some-resource",
						Source: atc.Source{"some": "source"},
					},
					Version: atc.Version{"some": "version"},
				},
				{
					ResourceType: atc.ResourceType{
						Name:   "custom-type-c",
						Type:   "custom-type-b",
						Source: atc.Source{"some": "source"},
					},
					Version: atc.Version{"some": "version"},
				},
				{
					ResourceType: atc.ResourceType{
						Name:   "custom-type-d",
						Type:   "custom-type-b",
						Source: atc.Source{"some": "source"},
					},
					Version: atc.Version{"some": "version"},
				},
				{
					ResourceType: atc.ResourceType{
						Name:   "unknown-custom-type",
						Type:   "unknown-base-type",
						Source: atc.Source{"some": "source"},
					},
					Version: atc.Version{"some": "version"},
				},
			})
		})

		JustBeforeEach(func() {
			satisfyingWorker, satisfyingErr = gardenWorker.Satisfying(logger, spec, customTypes)
		})

		Context("when the platform is compatible", func() {
			BeforeEach(func() {
				spec.Platform = "some-platform"
			})

			Context("when no tags are specified", func() {
				BeforeEach(func() {
					spec.Tags = nil
				})

				It("returns ErrIncompatiblePlatform", func() {
					Expect(satisfyingErr).To(Equal(ErrMismatchedTags))
				})
			})

			Context("when the worker has no tags", func() {
				BeforeEach(func() {
					tags = []string{}
					spec.Tags = []string{}
				})

				It("returns the worker", func() {
					Expect(satisfyingWorker).To(Equal(gardenWorker))
				})

				It("returns no error", func() {
					Expect(satisfyingErr).NotTo(HaveOccurred())
				})
			})

			Context("when all of the requested tags are present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"some", "tags"}
				})

				It("returns the worker", func() {
					Expect(satisfyingWorker).To(Equal(gardenWorker))
				})

				It("returns no error", func() {
					Expect(satisfyingErr).NotTo(HaveOccurred())
				})
			})

			Context("when some of the requested tags are present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"some"}
				})

				It("returns the worker", func() {
					Expect(satisfyingWorker).To(Equal(gardenWorker))
				})

				It("returns no error", func() {
					Expect(satisfyingErr).NotTo(HaveOccurred())
				})
			})

			Context("when any of the requested tags are not present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"bogus", "tags"}
				})

				It("returns ErrMismatchedTags", func() {
					Expect(satisfyingErr).To(Equal(ErrMismatchedTags))
				})
			})
		})

		Context("when the platform is incompatible", func() {
			BeforeEach(func() {
				spec.Platform = "some-bogus-platform"
			})

			It("returns ErrIncompatiblePlatform", func() {
				Expect(satisfyingErr).To(Equal(ErrIncompatiblePlatform))
			})
		})

		Context("when the resource type is supported by the worker", func() {
			BeforeEach(func() {
				spec.ResourceType = "some-resource"
			})

			Context("when all of the requested tags are present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"some", "tags"}
				})

				It("returns the worker", func() {
					Expect(satisfyingWorker).To(Equal(gardenWorker))
				})

				It("returns no error", func() {
					Expect(satisfyingErr).NotTo(HaveOccurred())
				})
			})

			Context("when some of the requested tags are present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"some"}
				})

				It("returns the worker", func() {
					Expect(satisfyingWorker).To(Equal(gardenWorker))
				})

				It("returns no error", func() {
					Expect(satisfyingErr).NotTo(HaveOccurred())
				})
			})

			Context("when any of the requested tags are not present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"bogus", "tags"}
				})

				It("returns ErrMismatchedTags", func() {
					Expect(satisfyingErr).To(Equal(ErrMismatchedTags))
				})
			})
		})

		Context("when the resource type is a custom type supported by the worker", func() {
			BeforeEach(func() {
				spec.ResourceType = "custom-type-c"
			})

			It("returns the worker", func() {
				Expect(satisfyingWorker).To(Equal(gardenWorker))
			})

			It("returns no error", func() {
				Expect(satisfyingErr).NotTo(HaveOccurred())
			})
		})

		Context("when the resource type is a custom type that overrides one supported by the worker", func() {
			BeforeEach(func() {
				variables := template.StaticVariables{}

				customTypes = creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
					{
						ResourceType: atc.ResourceType{
							Name:   "some-resource",
							Type:   "some-resource",
							Source: atc.Source{"some": "source"},
						},
						Version: atc.Version{"some": "version"},
					},
				})

				spec.ResourceType = "some-resource"
			})

			It("returns the worker", func() {
				Expect(satisfyingWorker).To(Equal(gardenWorker))
			})

			It("returns no error", func() {
				Expect(satisfyingErr).NotTo(HaveOccurred())
			})
		})

		Context("when the resource type is a custom type that results in a circular dependency", func() {
			BeforeEach(func() {
				variables := template.StaticVariables{}

				customTypes = creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
					atc.VersionedResourceType{
						ResourceType: atc.ResourceType{
							Name:   "circle-a",
							Type:   "circle-b",
							Source: atc.Source{"some": "source"},
						},
						Version: atc.Version{"some": "version"},
					}, atc.VersionedResourceType{
						ResourceType: atc.ResourceType{
							Name:   "circle-b",
							Type:   "circle-c",
							Source: atc.Source{"some": "source"},
						},
						Version: atc.Version{"some": "version"},
					}, atc.VersionedResourceType{
						ResourceType: atc.ResourceType{
							Name:   "circle-c",
							Type:   "circle-a",
							Source: atc.Source{"some": "source"},
						},
						Version: atc.Version{"some": "version"},
					},
				})

				spec.ResourceType = "circle-a"
			})

			It("returns ErrUnsupportedResourceType", func() {
				Expect(satisfyingErr).To(Equal(ErrUnsupportedResourceType))
			})
		})

		Context("when the resource type is a custom type not supported by the worker", func() {
			BeforeEach(func() {
				spec.ResourceType = "unknown-custom-type"
			})

			It("returns ErrUnsupportedResourceType", func() {
				Expect(satisfyingErr).To(Equal(ErrUnsupportedResourceType))
			})
		})

		Context("when the type is not supported by the worker", func() {
			BeforeEach(func() {
				spec.ResourceType = "some-other-resource"
			})

			It("returns ErrUnsupportedResourceType", func() {
				Expect(satisfyingErr).To(Equal(ErrUnsupportedResourceType))
			})
		})

		Context("when spec specifies team", func() {
			BeforeEach(func() {
				teamID = 123
				spec.TeamID = teamID
			})

			Context("when worker belongs to same team", func() {
				It("returns the worker", func() {
					Expect(satisfyingWorker).To(Equal(gardenWorker))
				})

				It("returns no error", func() {
					Expect(satisfyingErr).NotTo(HaveOccurred())
				})
			})

			Context("when worker belongs to different team", func() {
				BeforeEach(func() {
					teamID = 777
				})

				It("returns ErrTeamMismatch", func() {
					Expect(satisfyingErr).To(Equal(ErrTeamMismatch))
				})
			})

			Context("when worker does not belong to any team", func() {
				It("returns the worker", func() {
					Expect(satisfyingWorker).To(Equal(gardenWorker))
				})

				It("returns no error", func() {
					Expect(satisfyingErr).NotTo(HaveOccurred())
				})
			})
		})

		Context("when spec does not specify a team", func() {
			Context("when worker belongs to no team", func() {
				BeforeEach(func() {
					teamID = 0
				})

				It("returns the worker", func() {
					Expect(satisfyingWorker).To(Equal(gardenWorker))
				})

				It("returns no error", func() {
					Expect(satisfyingErr).NotTo(HaveOccurred())
				})
			})

			Context("when worker belongs to any team", func() {
				BeforeEach(func() {
					teamID = 555
				})

				It("returns ErrTeamMismatch", func() {
					Expect(satisfyingErr).To(Equal(ErrTeamMismatch))
				})
			})
		})
	})
})
