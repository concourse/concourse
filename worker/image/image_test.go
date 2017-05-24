package image_test

import (
	"io/ioutil"
	"strings"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/dbng/dbngfakes"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/image"
	"github.com/concourse/atc/worker/image/imagefakes"
	"github.com/concourse/atc/worker/workerfakes"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/baggageclaimfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Image", func() {
	var (
		imageFactory                    worker.ImageFactory
		img                             worker.Image
		logger                          *lagertest.TestLogger
		fakeWorker                      *workerfakes.FakeWorker
		fakeVolumeClient                *workerfakes.FakeVolumeClient
		fakeContainer                   *dbngfakes.FakeCreatingContainer
		fakeImageFetchingDelegate       *workerfakes.FakeImageFetchingDelegate
		fakeImageResourceFetcherFactory *imagefakes.FakeImageResourceFetcherFactory
		fakeImageResourceFetcher        *imagefakes.FakeImageResourceFetcher
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("image-tests")
		fakeWorker = new(workerfakes.FakeWorker)
		fakeVolumeClient = new(workerfakes.FakeVolumeClient)
		fakeContainer = new(dbngfakes.FakeCreatingContainer)
		fakeImageFetchingDelegate = new(workerfakes.FakeImageFetchingDelegate)

		fakeImageResourceFetcherFactory = new(imagefakes.FakeImageResourceFetcherFactory)
		fakeImageResourceFetcher = new(imagefakes.FakeImageResourceFetcher)
		fakeImageResourceFetcherFactory.ImageResourceFetcherForReturns(fakeImageResourceFetcher)
		imageFactory = image.NewImageFactory(fakeImageResourceFetcherFactory)
	})

	Describe("imageProvidedByPreviousStepOnSameWorker", func() {
		var fakeArtifactVolume *workerfakes.FakeVolume
		var cowStrategy baggageclaim.COWStrategy

		BeforeEach(func() {
			fakeArtifactVolume = new(workerfakes.FakeVolume)
			cowStrategy = baggageclaim.COWStrategy{
				Parent: new(baggageclaimfakes.FakeVolume),
			}
			fakeArtifactVolume.COWStrategyReturns(cowStrategy)

			fakeImageArtifactSource := new(workerfakes.FakeArtifactSource)
			fakeImageArtifactSource.VolumeOnReturns(fakeArtifactVolume, true, nil)
			metadataReader := ioutil.NopCloser(strings.NewReader(
				`{"env": ["A=1", "B=2"], "user":"image-volume-user"}`,
			))
			fakeImageArtifactSource.StreamFileReturns(metadataReader, nil)

			fakeContainerRootfsVolume := new(workerfakes.FakeVolume)
			fakeContainerRootfsVolume.PathReturns("some-path")
			fakeVolumeClient.FindOrCreateCOWVolumeForContainerReturns(fakeContainerRootfsVolume, nil)

			var err error
			img, err = imageFactory.GetImage(
				logger,
				fakeWorker,
				fakeVolumeClient,
				worker.ImageSpec{
					ImageArtifactSource: fakeImageArtifactSource,
					Privileged:          true,
				},
				42,
				nil,
				fakeImageFetchingDelegate,
				dbng.ForBuild(42),
				atc.VersionedResourceTypes{},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("finds or creates cow volume", func() {
			_, err := img.FetchForContainer(logger, fakeContainer)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeVolumeClient.FindOrCreateCOWVolumeForContainerCallCount()).To(Equal(1))
			_, volumeSpec, container, volume, teamID, path := fakeVolumeClient.FindOrCreateCOWVolumeForContainerArgsForCall(0)
			Expect(volumeSpec).To(Equal(worker.VolumeSpec{
				Strategy:   cowStrategy,
				Privileged: true,
			}))
			Expect(container).To(Equal(fakeContainer))
			Expect(volume).To(Equal(fakeArtifactVolume))
			Expect(teamID).To(Equal(42))
			Expect(path).To(Equal("/"))
		})

		It("returns fetched image", func() {
			fetchedImage, err := img.FetchForContainer(logger, fakeContainer)
			Expect(err).NotTo(HaveOccurred())

			Expect(fetchedImage).To(Equal(worker.FetchedImage{
				Metadata: worker.ImageMetadata{
					Env:  []string{"A=1", "B=2"},
					User: "image-volume-user",
				},
				URL:        "raw://some-path/rootfs",
				Privileged: true,
			}))
		})
	})

	Describe("imageProvidedByPreviousStepOnDifferentWorker", func() {
		var (
			fakeArtifactVolume        *workerfakes.FakeVolume
			fakeImageArtifactSource   *workerfakes.FakeArtifactSource
			fakeContainerRootfsVolume *workerfakes.FakeVolume
		)

		BeforeEach(func() {
			fakeArtifactVolume = new(workerfakes.FakeVolume)
			fakeImageArtifactSource = new(workerfakes.FakeArtifactSource)
			fakeImageArtifactSource.VolumeOnReturns(fakeArtifactVolume, false, nil)
			metadataReader := ioutil.NopCloser(strings.NewReader(
				`{"env": ["A=1", "B=2"], "user":"image-volume-user"}`,
			))
			fakeImageArtifactSource.StreamFileReturns(metadataReader, nil)

			fakeContainerRootfsVolume = new(workerfakes.FakeVolume)
			fakeContainerRootfsVolume.PathReturns("some-path")
			fakeVolumeClient.FindOrCreateVolumeForContainerReturns(fakeContainerRootfsVolume, nil)

			var err error
			img, err = imageFactory.GetImage(
				logger,
				fakeWorker,
				fakeVolumeClient,
				worker.ImageSpec{
					ImageArtifactSource: fakeImageArtifactSource,
					ImageArtifactName:   "some-image-artifact-name",
					Privileged:          true,
				},
				42,
				nil,
				fakeImageFetchingDelegate,
				dbng.ForBuild(42),
				atc.VersionedResourceTypes{},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("finds or creates volume", func() {
			_, err := img.FetchForContainer(logger, fakeContainer)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeVolumeClient.FindOrCreateVolumeForContainerCallCount()).To(Equal(1))
			_, volumeSpec, container, teamID, path := fakeVolumeClient.FindOrCreateVolumeForContainerArgsForCall(0)
			Expect(volumeSpec).To(Equal(worker.VolumeSpec{
				Strategy:   baggageclaim.EmptyStrategy{},
				Privileged: true,
			}))
			Expect(container).To(Equal(fakeContainer))
			Expect(teamID).To(Equal(42))
			Expect(path).To(Equal("/"))
		})

		It("streams the volume from another worker", func() {
			_, err := img.FetchForContainer(logger, fakeContainer)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeImageArtifactSource.StreamToCallCount()).To(Equal(1))

			artifactDestination := fakeImageArtifactSource.StreamToArgsForCall(0)
			artifactDestination.StreamIn("fake-path", strings.NewReader("fake-tar-stream"))
			Expect(fakeContainerRootfsVolume.StreamInCallCount()).To(Equal(1))
		})

		It("returns fetched image", func() {
			fetchedImage, err := img.FetchForContainer(logger, fakeContainer)
			Expect(err).NotTo(HaveOccurred())

			Expect(fetchedImage).To(Equal(worker.FetchedImage{
				Metadata: worker.ImageMetadata{
					Env:  []string{"A=1", "B=2"},
					User: "image-volume-user",
				},
				URL:        "raw://some-path/rootfs",
				Privileged: true,
			}))
		})
	})

	Describe("imageFromResource", func() {
		var imageSpec worker.ImageSpec
		var resourceTypes atc.VersionedResourceTypes

		var fakeResourceImageVolume *workerfakes.FakeVolume
		var cowStrategy baggageclaim.COWStrategy
		var fakeContainerRootfsVolume *workerfakes.FakeVolume

		BeforeEach(func() {
			imageSpec = worker.ImageSpec{
				ImageResource: &atc.ImageResource{
					Type:   "some-image-resource-type",
					Source: atc.Source{"some": "source"},
				},
				Privileged: true,
			}

			resourceTypes = atc.VersionedResourceTypes{
				{
					ResourceType: atc.ResourceType{
						Name:   "some-type",
						Type:   "docker-image",
						Source: atc.Source{"some": "source"},
					},
					Version: atc.Version{"some": "version"},
				},
				{
					ResourceType: atc.ResourceType{
						Name:       "some-image-type",
						Type:       "docker-image",
						Source:     atc.Source{"some": "image-source"},
						Privileged: true,
					},
					Version: atc.Version{"some": "image-version"},
				},
			}

			metadataReader := ioutil.NopCloser(strings.NewReader(
				`{"env": ["A=1", "B=2"], "user":"image-volume-user"}`,
			))

			fakeResourceImageVolume = new(workerfakes.FakeVolume)
			cowStrategy = baggageclaim.COWStrategy{
				Parent: new(baggageclaimfakes.FakeVolume),
			}
			fakeResourceImageVolume.COWStrategyReturns(cowStrategy)

			fakeContainerRootfsVolume = new(workerfakes.FakeVolume)
			fakeContainerRootfsVolume.PathReturns("some-path")
			fakeVolumeClient.FindOrCreateCOWVolumeForContainerReturns(fakeContainerRootfsVolume, nil)

			fakeImageResourceFetcher.FetchReturns(
				fakeResourceImageVolume,
				metadataReader,
				atc.Version{"some": "version"},
				nil,
			)
		})

		JustBeforeEach(func() {
			var err error
			img, err = imageFactory.GetImage(
				logger,
				fakeWorker,
				fakeVolumeClient,
				imageSpec,
				42,
				nil,
				fakeImageFetchingDelegate,
				dbng.ForBuild(42),
				resourceTypes,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("finds or creates cow volume", func() {
			_, err := img.FetchForContainer(logger, fakeContainer)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeVolumeClient.FindOrCreateCOWVolumeForContainerCallCount()).To(Equal(1))
			_, volumeSpec, container, volume, teamID, path := fakeVolumeClient.FindOrCreateCOWVolumeForContainerArgsForCall(0)
			Expect(volumeSpec).To(Equal(worker.VolumeSpec{
				Strategy:   cowStrategy,
				Privileged: true,
			}))
			Expect(container).To(Equal(fakeContainer))
			Expect(volume).To(Equal(fakeResourceImageVolume))
			Expect(teamID).To(Equal(42))
			Expect(path).To(Equal("/"))
		})

		It("returns fetched image", func() {
			fetchedImage, err := img.FetchForContainer(logger, fakeContainer)
			Expect(err).NotTo(HaveOccurred())

			Expect(fetchedImage).To(Equal(worker.FetchedImage{
				Metadata: worker.ImageMetadata{
					Env:  []string{"A=1", "B=2"},
					User: "image-volume-user",
				},
				URL:        "raw://some-path/rootfs",
				Version:    atc.Version{"some": "version"},
				Privileged: true,
			}))
		})

		Context("from a custom resource type", func() {
			Context("unprivileged", func() {
				BeforeEach(func() {
					imageSpec = worker.ImageSpec{
						ResourceType: "some-type",
					}
				})

				It("finds or creates cow volume", func() {
					_, err := img.FetchForContainer(logger, fakeContainer)
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeVolumeClient.FindOrCreateCOWVolumeForContainerCallCount()).To(Equal(1))
					_, volumeSpec, container, volume, teamID, path := fakeVolumeClient.FindOrCreateCOWVolumeForContainerArgsForCall(0)
					Expect(volumeSpec).To(Equal(worker.VolumeSpec{
						Strategy:   cowStrategy,
						Privileged: false,
					}))
					Expect(container).To(Equal(fakeContainer))
					Expect(volume).To(Equal(fakeResourceImageVolume))
					Expect(teamID).To(Equal(42))
					Expect(path).To(Equal("/"))
				})

				It("returns fetched image", func() {
					fetchedImage, err := img.FetchForContainer(logger, fakeContainer)
					Expect(err).NotTo(HaveOccurred())

					Expect(fetchedImage).To(Equal(worker.FetchedImage{
						Metadata: worker.ImageMetadata{
							Env:  []string{"A=1", "B=2"},
							User: "image-volume-user",
						},
						URL:        "raw://some-path/rootfs",
						Version:    atc.Version{"some": "version"},
						Privileged: false,
					}))
				})
			})

			Context("privileged", func() {
				BeforeEach(func() {
					imageSpec = worker.ImageSpec{
						ResourceType: "some-image-type",
					}
				})

				It("finds or creates cow volume", func() {
					_, err := img.FetchForContainer(logger, fakeContainer)
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeVolumeClient.FindOrCreateCOWVolumeForContainerCallCount()).To(Equal(1))
					_, volumeSpec, container, volume, teamID, path := fakeVolumeClient.FindOrCreateCOWVolumeForContainerArgsForCall(0)
					Expect(volumeSpec).To(Equal(worker.VolumeSpec{
						Strategy:   cowStrategy,
						Privileged: true,
					}))
					Expect(container).To(Equal(fakeContainer))
					Expect(volume).To(Equal(fakeResourceImageVolume))
					Expect(teamID).To(Equal(42))
					Expect(path).To(Equal("/"))
				})

				It("returns fetched image", func() {
					fetchedImage, err := img.FetchForContainer(logger, fakeContainer)
					Expect(err).NotTo(HaveOccurred())

					Expect(fetchedImage).To(Equal(worker.FetchedImage{
						Metadata: worker.ImageMetadata{
							Env:  []string{"A=1", "B=2"},
							User: "image-volume-user",
						},
						URL:        "raw://some-path/rootfs",
						Version:    atc.Version{"some": "version"},
						Privileged: true,
					}))
				})
			})
		})
	})

	Describe("imageFromBaseResourceType", func() {
		var cowStrategy baggageclaim.COWStrategy
		var workerResourceType atc.WorkerResourceType
		var fakeContainerRootfsVolume *workerfakes.FakeVolume
		var fakeImportVolume *workerfakes.FakeVolume

		BeforeEach(func() {
			fakeContainerRootfsVolume = new(workerfakes.FakeVolume)
			fakeContainerRootfsVolume.PathReturns("some-path")
			fakeVolumeClient.FindOrCreateCOWVolumeForContainerReturns(fakeContainerRootfsVolume, nil)

			fakeImportVolume = new(workerfakes.FakeVolume)
			cowStrategy = baggageclaim.COWStrategy{
				Parent: new(baggageclaimfakes.FakeVolume),
			}
			fakeImportVolume.COWStrategyReturns(cowStrategy)
			fakeVolumeClient.FindOrCreateVolumeForBaseResourceTypeReturns(fakeImportVolume, nil)

			workerResourceType = atc.WorkerResourceType{
				Type:       "some-base-resource-type",
				Image:      "some-base-image-path",
				Version:    "some-base-version",
				Privileged: false,
			}

			fakeWorker.ResourceTypesReturns([]atc.WorkerResourceType{
				workerResourceType,
			})

			fakeWorker.NameReturns("some-worker-name")

			var err error
			img, err = imageFactory.GetImage(
				logger,
				fakeWorker,
				fakeVolumeClient,
				worker.ImageSpec{
					ResourceType: "some-base-resource-type",
				},
				42,
				nil,
				fakeImageFetchingDelegate,
				dbng.ForBuild(42),
				atc.VersionedResourceTypes{},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("finds or creates unprivileged import volume", func() {
			_, err := img.FetchForContainer(logger, fakeContainer)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeVolumeClient.FindOrCreateVolumeForBaseResourceTypeCallCount()).To(Equal(1))
			_, volumeSpec, teamID, resourceTypeName := fakeVolumeClient.FindOrCreateVolumeForBaseResourceTypeArgsForCall(0)
			Expect(volumeSpec).To(Equal(worker.VolumeSpec{
				Strategy: baggageclaim.ImportStrategy{
					Path: "some-base-image-path",
				},
				Privileged: false,
			}))
			Expect(teamID).To(Equal(42))
			Expect(resourceTypeName).To(Equal("some-base-resource-type"))
		})

		It("finds or creates unprivileged cow volume", func() {
			_, err := img.FetchForContainer(logger, fakeContainer)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeVolumeClient.FindOrCreateCOWVolumeForContainerCallCount()).To(Equal(1))
			_, volumeSpec, container, volume, teamID, path := fakeVolumeClient.FindOrCreateCOWVolumeForContainerArgsForCall(0)
			Expect(volumeSpec).To(Equal(worker.VolumeSpec{
				Strategy:   cowStrategy,
				Privileged: false,
			}))
			Expect(teamID).To(Equal(42))
			Expect(container).To(Equal(fakeContainer))
			Expect(volume).To(Equal(fakeImportVolume))
			Expect(path).To(Equal("/"))
		})

		It("returns fetched image", func() {
			fetchedImage, err := img.FetchForContainer(logger, fakeContainer)
			Expect(err).NotTo(HaveOccurred())

			Expect(fetchedImage).To(Equal(worker.FetchedImage{
				Metadata:   worker.ImageMetadata{},
				URL:        "raw://some-path",
				Version:    atc.Version{"some-base-resource-type": "some-base-version"},
				Privileged: false,
			}))
		})

		Context("when the worker base resource type is privileged", func() {
			BeforeEach(func() {
				workerResourceType.Privileged = true
				fakeWorker.ResourceTypesReturns([]atc.WorkerResourceType{workerResourceType})
			})

			It("finds or creates privileged import volume", func() {
				_, err := img.FetchForContainer(logger, fakeContainer)
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeVolumeClient.FindOrCreateVolumeForBaseResourceTypeCallCount()).To(Equal(1))
				_, volumeSpec, teamID, resourceTypeName := fakeVolumeClient.FindOrCreateVolumeForBaseResourceTypeArgsForCall(0)
				Expect(volumeSpec).To(Equal(worker.VolumeSpec{
					Strategy: baggageclaim.ImportStrategy{
						Path: "some-base-image-path",
					},
					Privileged: true,
				}))
				Expect(teamID).To(Equal(42))
				Expect(resourceTypeName).To(Equal("some-base-resource-type"))
			})

			It("finds or creates privileged cow volume", func() {
				_, err := img.FetchForContainer(logger, fakeContainer)
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeVolumeClient.FindOrCreateCOWVolumeForContainerCallCount()).To(Equal(1))
				_, volumeSpec, container, volume, teamID, path := fakeVolumeClient.FindOrCreateCOWVolumeForContainerArgsForCall(0)
				Expect(volumeSpec).To(Equal(worker.VolumeSpec{
					Strategy:   cowStrategy,
					Privileged: true,
				}))
				Expect(teamID).To(Equal(42))
				Expect(container).To(Equal(fakeContainer))
				Expect(volume).To(Equal(fakeImportVolume))
				Expect(path).To(Equal("/"))
			})

			It("returns privileged fetched image", func() {
				fetchedImage, err := img.FetchForContainer(logger, fakeContainer)
				Expect(err).NotTo(HaveOccurred())

				Expect(fetchedImage).To(Equal(worker.FetchedImage{
					Metadata:   worker.ImageMetadata{},
					URL:        "raw://some-path",
					Version:    atc.Version{"some-base-resource-type": "some-base-version"},
					Privileged: true,
				}))
			})
		})
	})

	Describe("imageFromRootfsURI", func() {
		BeforeEach(func() {
			var err error
			img, err = imageFactory.GetImage(
				logger,
				fakeWorker,
				fakeVolumeClient,
				worker.ImageSpec{
					ImageURL: "some-image-url",
				},
				42,
				nil,
				fakeImageFetchingDelegate,
				dbng.ForBuild(42),
				atc.VersionedResourceTypes{},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the fetched image", func() {
			fetchedImage, err := img.FetchForContainer(logger, fakeContainer)
			Expect(err).NotTo(HaveOccurred())

			Expect(fetchedImage).To(Equal(worker.FetchedImage{
				URL: "some-image-url",
			}))
		})
	})
})
