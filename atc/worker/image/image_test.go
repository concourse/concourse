package image_test

import (
	"context"
	"io/ioutil"
	"strings"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db/dbfakes"
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
		ctx                             context.Context
		logger                          *lagertest.TestLogger
		fakeWorker                      *workerfakes.FakeWorker
		fakeVolumeClient                *workerfakes.FakeVolumeClient
		fakeContainer                   *dbfakes.FakeCreatingContainer
		fakeImageFetchingDelegate       *workerfakes.FakeImageFetchingDelegate
		fakeImageResourceFetcherFactory *imagefakes.FakeImageResourceFetcherFactory
		fakeImageResourceFetcher        *imagefakes.FakeImageResourceFetcher
		variables                       creds.Variables
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("image-tests")
		fakeWorker = new(workerfakes.FakeWorker)
		fakeWorker.TagsReturns(atc.Tags{"worker", "tags"})

		ctx = context.Background()
		fakeVolumeClient = new(workerfakes.FakeVolumeClient)
		fakeContainer = new(dbfakes.FakeCreatingContainer)
		fakeImageFetchingDelegate = new(workerfakes.FakeImageFetchingDelegate)

		fakeImageResourceFetcherFactory = new(imagefakes.FakeImageResourceFetcherFactory)
		fakeImageResourceFetcher = new(imagefakes.FakeImageResourceFetcher)
		fakeImageResourceFetcherFactory.NewImageResourceFetcherReturns(fakeImageResourceFetcher)
		imageFactory = image.NewImageFactory(fakeImageResourceFetcherFactory)

		variables = template.StaticVariables{
			"source-secret": "super-secret-sauce",
		}
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
				fakeImageFetchingDelegate,
				creds.VersionedResourceTypes{},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("finds or creates cow volume", func() {
			_, err := img.FetchForContainer(ctx, logger, fakeContainer)
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
			fetchedImage, err := img.FetchForContainer(ctx, logger, fakeContainer)
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
				fakeImageFetchingDelegate,
				creds.VersionedResourceTypes{},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("finds or creates volume", func() {
			_, err := img.FetchForContainer(ctx, logger, fakeContainer)
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
			_, err := img.FetchForContainer(ctx, logger, fakeContainer)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeImageArtifactSource.StreamToCallCount()).To(Equal(1))

			artifactDestination := fakeImageArtifactSource.StreamToArgsForCall(0)
			artifactDestination.StreamIn("fake-path", strings.NewReader("fake-tar-stream"))
			Expect(fakeContainerRootfsVolume.StreamInCallCount()).To(Equal(1))
		})

		It("returns fetched image", func() {
			fetchedImage, err := img.FetchForContainer(ctx, logger, fakeContainer)
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
		var fakeResourceImageVolume *workerfakes.FakeVolume
		var cowStrategy baggageclaim.COWStrategy
		var fakeContainerRootfsVolume *workerfakes.FakeVolume

		BeforeEach(func() {
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

		Context("when image is provided as image resource", func() {
			BeforeEach(func() {
				var err error
				img, err = imageFactory.GetImage(
					logger,
					fakeWorker,
					fakeVolumeClient,
					worker.ImageSpec{
						ImageResource: &worker.ImageResource{
							Type:   "some-image-resource-type",
							Source: creds.NewSource(variables, atc.Source{"some": "source"}),
						},
						Privileged: true,
					},
					42,
					fakeImageFetchingDelegate,
					creds.VersionedResourceTypes{},
				)
				Expect(err).NotTo(HaveOccurred())
			})

			It("fetches image without custom resource type", func() {
				worker, _, imageResource, version, teamID, resourceTypes, delegate := fakeImageResourceFetcherFactory.NewImageResourceFetcherArgsForCall(0)
				Expect(worker).To(Equal(fakeWorker))
				Expect(imageResource.Type).To(Equal("some-image-resource-type"))
				Expect(imageResource.Source).To(Equal(creds.NewSource(variables, atc.Source{"some": "source"})))
				Expect(version).To(BeNil())
				Expect(teamID).To(Equal(42))
				Expect(resourceTypes).To(Equal(creds.VersionedResourceTypes{}))
				Expect(delegate).To(Equal(fakeImageFetchingDelegate))
			})

			It("finds or creates cow volume", func() {
				_, err := img.FetchForContainer(ctx, logger, fakeContainer)
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
				fetchedImage, err := img.FetchForContainer(ctx, logger, fakeContainer)
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

		Context("when image is provided as unprivileged custom resource type", func() {
			BeforeEach(func() {
				var err error
				img, err = imageFactory.GetImage(
					logger,
					fakeWorker,
					fakeVolumeClient,
					worker.ImageSpec{
						ResourceType: "some-custom-resource-type",
					},
					42,
					fakeImageFetchingDelegate,
					creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
						{
							ResourceType: atc.ResourceType{
								Name: "some-custom-resource-type",
								Type: "some-base-resource-type",
								Source: atc.Source{
									"some": "custom-resource-type-source",
								},
							},
							Version: atc.Version{"some": "custom-resource-type-version"},
						},
						{
							ResourceType: atc.ResourceType{
								Name: "some-custom-image-resource-type",
								Type: "some-base-image-resource-type",
								Source: atc.Source{
									"some": "custom-image-resource-type-source",
								},
								Privileged: true,
							},
							Version: atc.Version{"some": "custom-image-resource-type-version"},
						},
					}),
				)
				Expect(err).NotTo(HaveOccurred())
			})

			It("fetches unprivileged image without custom resource type", func() {
				worker, _, imageResource, version, teamID, resourceTypes, delegate := fakeImageResourceFetcherFactory.NewImageResourceFetcherArgsForCall(0)
				Expect(worker).To(Equal(fakeWorker))
				Expect(imageResource.Type).To(Equal("some-base-resource-type"))
				Expect(imageResource.Source).To(Equal(creds.NewSource(variables, atc.Source{
					"some": "custom-resource-type-source",
				})))
				Expect(version).To(Equal(atc.Version{"some": "custom-resource-type-version"}))
				Expect(teamID).To(Equal(42))
				Expect(resourceTypes).To(Equal(creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
					{
						ResourceType: atc.ResourceType{
							Name: "some-custom-image-resource-type",
							Type: "some-base-image-resource-type",
							Source: atc.Source{
								"some": "custom-image-resource-type-source",
							},
							Privileged: true,
						},
						Version: atc.Version{"some": "custom-image-resource-type-version"},
					},
				})))
				Expect(delegate).To(Equal(fakeImageFetchingDelegate))
			})

			It("finds or creates unprivileged cow volume", func() {
				_, err := img.FetchForContainer(ctx, logger, fakeContainer)
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
				fetchedImage, err := img.FetchForContainer(ctx, logger, fakeContainer)
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

		Context("when image is provided as privileged custom resource type", func() {
			BeforeEach(func() {
				var err error
				img, err = imageFactory.GetImage(
					logger,
					fakeWorker,
					fakeVolumeClient,
					worker.ImageSpec{
						ResourceType: "some-custom-image-resource-type",
					},
					42,
					fakeImageFetchingDelegate,
					creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
						{
							ResourceType: atc.ResourceType{
								Name: "some-custom-resource-type",
								Type: "some-base-resource-type",
								Source: atc.Source{
									"some": "custom-resource-type-source",
								},
							},
							Version: atc.Version{"some": "custom-resource-type-version"},
						},
						{
							ResourceType: atc.ResourceType{
								Name: "some-custom-image-resource-type",
								Type: "some-base-image-resource-type",
								Source: atc.Source{
									"some": "custom-image-resource-type-source",
								},
								Privileged: true,
							},
							Version: atc.Version{"some": "custom-image-resource-type-version"},
						},
					}),
				)
				Expect(err).NotTo(HaveOccurred())
			})

			It("fetches image without custom resource type", func() {
				worker, _, imageResource, version, teamID, resourceTypes, delegate := fakeImageResourceFetcherFactory.NewImageResourceFetcherArgsForCall(0)
				Expect(worker).To(Equal(fakeWorker))
				Expect(imageResource.Type).To(Equal("some-base-image-resource-type"))
				Expect(imageResource.Source).To(Equal(creds.NewSource(variables, atc.Source{
					"some": "custom-image-resource-type-source",
				})))
				Expect(version).To(Equal(atc.Version{"some": "custom-image-resource-type-version"}))
				Expect(teamID).To(Equal(42))
				Expect(resourceTypes).To(Equal(creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
					{
						ResourceType: atc.ResourceType{
							Name: "some-custom-resource-type",
							Type: "some-base-resource-type",
							Source: atc.Source{
								"some": "custom-resource-type-source",
							},
						},
						Version: atc.Version{"some": "custom-resource-type-version"},
					},
				})))
				Expect(delegate).To(Equal(fakeImageFetchingDelegate))
			})

			It("finds or creates cow volume", func() {
				_, err := img.FetchForContainer(ctx, logger, fakeContainer)
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
				fetchedImage, err := img.FetchForContainer(ctx, logger, fakeContainer)
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

			fakeWorker.ResourceTypesReturns([]atc.WorkerResourceType{workerResourceType})

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
				fakeImageFetchingDelegate,
				creds.VersionedResourceTypes{},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("finds or creates unprivileged import volume", func() {
			_, err := img.FetchForContainer(ctx, logger, fakeContainer)
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
			_, err := img.FetchForContainer(ctx, logger, fakeContainer)
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
			fetchedImage, err := img.FetchForContainer(ctx, logger, fakeContainer)
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
				_, err := img.FetchForContainer(ctx, logger, fakeContainer)
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
				_, err := img.FetchForContainer(ctx, logger, fakeContainer)
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
				fetchedImage, err := img.FetchForContainer(ctx, logger, fakeContainer)
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
				fakeImageFetchingDelegate,
				creds.VersionedResourceTypes{},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the fetched image", func() {
			fetchedImage, err := img.FetchForContainer(ctx, logger, fakeContainer)
			Expect(err).NotTo(HaveOccurred())

			Expect(fetchedImage).To(Equal(worker.FetchedImage{
				URL: "some-image-url",
			}))
		})
	})
})
