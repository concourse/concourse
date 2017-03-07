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

		BeforeEach(func() {
			fakeArtifactVolume = new(workerfakes.FakeVolume)
			fakeImageArtifactSource := new(workerfakes.FakeArtifactSource)
			fakeImageArtifactSource.VolumeOnReturns(fakeArtifactVolume, true, nil)
			metadataReader := ioutil.NopCloser(strings.NewReader(
				`{"env": ["A=1", "B=2"], "user":"image-volume-user"}`,
			))
			fakeImageArtifactSource.StreamFileReturns(metadataReader, nil)

			fakeImageVolume := new(workerfakes.FakeVolume)
			fakeImageVolume.PathReturns("some-path")
			fakeVolumeClient.FindOrCreateVolumeForContainerReturns(fakeImageVolume, nil)

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
				dbng.ForBuild{BuildID: 42},
				worker.Identifier{},
				worker.Metadata{},
				atc.VersionedResourceTypes{},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("finds or creates volume with ContainerRootFSStrategy", func() {
			_, err := img.FetchForContainer(logger, fakeContainer)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeVolumeClient.FindOrCreateVolumeForContainerCallCount()).To(Equal(1))
			_, volumeSpec, container, teamID, path := fakeVolumeClient.FindOrCreateVolumeForContainerArgsForCall(0)
			Expect(volumeSpec).To(Equal(worker.VolumeSpec{
				Strategy: worker.ContainerRootFSStrategy{
					Parent: fakeArtifactVolume,
				},
				Privileged: true,
			}))
			Expect(container).To(Equal(fakeContainer))
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
				URL: "raw://some-path/rootfs",
			}))
		})
	})

	Describe("imageProvidedByPreviousStepOnDifferentWorker", func() {
		var (
			fakeArtifactVolume      *workerfakes.FakeVolume
			fakeImageArtifactSource *workerfakes.FakeArtifactSource
			fakeImageVolume         *workerfakes.FakeVolume
		)

		BeforeEach(func() {
			fakeArtifactVolume = new(workerfakes.FakeVolume)
			fakeImageArtifactSource = new(workerfakes.FakeArtifactSource)
			fakeImageArtifactSource.VolumeOnReturns(fakeArtifactVolume, false, nil)
			metadataReader := ioutil.NopCloser(strings.NewReader(
				`{"env": ["A=1", "B=2"], "user":"image-volume-user"}`,
			))
			fakeImageArtifactSource.StreamFileReturns(metadataReader, nil)

			fakeImageVolume = new(workerfakes.FakeVolume)
			fakeImageVolume.PathReturns("some-path")
			fakeVolumeClient.FindOrCreateVolumeForContainerReturns(fakeImageVolume, nil)

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
				dbng.ForBuild{BuildID: 42},
				worker.Identifier{},
				worker.Metadata{},
				atc.VersionedResourceTypes{},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("finds or creates volume with ImageArtifactReplicationStrategy", func() {
			_, err := img.FetchForContainer(logger, fakeContainer)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeVolumeClient.FindOrCreateVolumeForContainerCallCount()).To(Equal(1))
			_, volumeSpec, container, teamID, path := fakeVolumeClient.FindOrCreateVolumeForContainerArgsForCall(0)
			Expect(volumeSpec).To(Equal(worker.VolumeSpec{
				Strategy: worker.ImageArtifactReplicationStrategy{
					Name: "some-image-artifact-name",
				},
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
			Expect(fakeImageVolume.StreamInCallCount()).To(Equal(1))
		})

		It("returns fetched image", func() {
			fetchedImage, err := img.FetchForContainer(logger, fakeContainer)
			Expect(err).NotTo(HaveOccurred())

			Expect(fetchedImage).To(Equal(worker.FetchedImage{
				Metadata: worker.ImageMetadata{
					Env:  []string{"A=1", "B=2"},
					User: "image-volume-user",
				},
				URL: "raw://some-path/rootfs",
			}))
		})
	})

	Describe("imageFromResource", func() {
		var fakeResourceImageVolume *workerfakes.FakeVolume

		BeforeEach(func() {
			metadataReader := ioutil.NopCloser(strings.NewReader(
				`{"env": ["A=1", "B=2"], "user":"image-volume-user"}`,
			))

			fakeResourceImageVolume = new(workerfakes.FakeVolume)
			fakeResourceImageVolume.PathReturns("some-path")
			fakeVolumeClient.FindOrCreateVolumeForContainerReturns(fakeResourceImageVolume, nil)

			fakeImageResourceFetcher.FetchReturns(
				fakeResourceImageVolume,
				metadataReader,
				atc.Version{"some": "version"},
				nil,
			)

			var err error
			img, err = imageFactory.GetImage(
				logger,
				fakeWorker,
				fakeVolumeClient,
				worker.ImageSpec{
					ImageResource: &atc.ImageResource{
						Type:   "some-image-resource-type",
						Source: atc.Source{"some": "source"},
					},
					Privileged: true,
				},
				42,
				nil,
				fakeImageFetchingDelegate,
				dbng.ForBuild{BuildID: 42},
				worker.Identifier{},
				worker.Metadata{},
				atc.VersionedResourceTypes{},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("finds or creates volume with ContainerRootFSStrategy", func() {
			_, err := img.FetchForContainer(logger, fakeContainer)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeVolumeClient.FindOrCreateVolumeForContainerCallCount()).To(Equal(1))
			_, volumeSpec, container, teamID, path := fakeVolumeClient.FindOrCreateVolumeForContainerArgsForCall(0)
			Expect(volumeSpec).To(Equal(worker.VolumeSpec{
				Strategy: worker.ContainerRootFSStrategy{
					Parent: fakeResourceImageVolume,
				},
				Privileged: true,
			}))
			Expect(container).To(Equal(fakeContainer))
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
				URL:     "raw://some-path/rootfs",
				Version: atc.Version{"some": "version"},
			}))
		})
	})

	Describe("imageFromBaseResourceType", func() {
		var fakeResourceImageVolume *workerfakes.FakeVolume

		BeforeEach(func() {
			fakeResourceImageVolume = new(workerfakes.FakeVolume)
			fakeResourceImageVolume.PathReturns("some-path")
			fakeVolumeClient.FindOrCreateVolumeForContainerReturns(fakeResourceImageVolume, nil)

			fakeWorker.ResourceTypesReturns([]atc.WorkerResourceType{
				{
					Type:    "some-base-resource-type",
					Image:   "some-base-image-path",
					Version: "some-base-version",
				},
			})

			fakeWorker.NameReturns("some-worker-name")

			var err error
			img, err = imageFactory.GetImage(
				logger,
				fakeWorker,
				fakeVolumeClient,
				worker.ImageSpec{
					ResourceType: "some-base-resource-type",
					Privileged:   true,
				},
				42,
				nil,
				fakeImageFetchingDelegate,
				dbng.ForBuild{BuildID: 42},
				worker.Identifier{},
				worker.Metadata{},
				atc.VersionedResourceTypes{},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("finds or creates import volume with HostRootFSStrategy", func() {
			_, err := img.FetchForContainer(logger, fakeContainer)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeVolumeClient.FindOrCreateVolumeForBaseResourceTypeCallCount()).To(Equal(1))
			_, volumeSpec, teamID, resourceTypeName := fakeVolumeClient.FindOrCreateVolumeForBaseResourceTypeArgsForCall(0)
			expectedVersion := "some-base-version"
			Expect(volumeSpec).To(Equal(worker.VolumeSpec{
				Strategy: worker.HostRootFSStrategy{
					Path:       "some-base-image-path",
					Version:    &expectedVersion,
					WorkerName: "some-worker-name",
				},
				Privileged: true,
				Properties: worker.VolumeProperties{},
			}))
			Expect(teamID).To(Equal(42))
			Expect(resourceTypeName).To(Equal("some-base-resource-type"))
		})

		It("finds or creates cow volume with ContainerRootFSStrategy", func() {
			fakeImportVolume := new(workerfakes.FakeVolume)
			fakeVolumeClient.FindOrCreateVolumeForBaseResourceTypeReturns(fakeImportVolume, nil)
			_, err := img.FetchForContainer(logger, fakeContainer)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeVolumeClient.FindOrCreateVolumeForContainerCallCount()).To(Equal(1))
			_, volumeSpec, container, teamID, path := fakeVolumeClient.FindOrCreateVolumeForContainerArgsForCall(0)
			Expect(volumeSpec).To(Equal(worker.VolumeSpec{
				Strategy: worker.ContainerRootFSStrategy{
					Parent: fakeImportVolume,
				},
				Privileged: true,
				Properties: worker.VolumeProperties{},
			}))
			Expect(teamID).To(Equal(42))
			Expect(container).To(Equal(fakeContainer))
			Expect(path).To(Equal("/"))
		})

		It("returns fetched image", func() {
			fetchedImage, err := img.FetchForContainer(logger, fakeContainer)
			Expect(err).NotTo(HaveOccurred())

			Expect(fetchedImage).To(Equal(worker.FetchedImage{
				Metadata: worker.ImageMetadata{},
				URL:      "raw://some-path",
				Version:  atc.Version{"some-base-resource-type": "some-base-version"},
			}))
		})
	})

	Describe("imageInTask", func() {
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
				dbng.ForBuild{BuildID: 42},
				worker.Identifier{},
				worker.Metadata{},
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
