package image_test

import (
	"archive/tar"
	"context"
	"errors"
	"io"
	"io/ioutil"

	"github.com/concourse/concourse/atc/resource"

	"github.com/concourse/concourse/atc/runtime"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/DataDog/zstd"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/resource/resourcefakes"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/image"
	"github.com/concourse/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Image", func() {
	var (
		fakeResourceFetcher *workerfakes.FakeFetcher

		fakeResourceFactory *resourcefakes.FakeResourceFactory

		fakeCheckResource *resourcefakes.FakeResource
		fakeGetResource   *resourcefakes.FakeResource

		fakeResourceCacheFactory  *dbfakes.FakeResourceCacheFactory
		fakeResourceConfigFactory *dbfakes.FakeResourceConfigFactory
		fakeCreatingContainer     *dbfakes.FakeCreatingContainer

		imageResourceFetcher image.ImageResourceFetcher

		stderrBuf *gbytes.Buffer

		logger                    lager.Logger
		imageResource             worker.ImageResource
		version                   atc.Version
		ctx                       context.Context
		fakeImageFetchingDelegate *workerfakes.FakeImageFetchingDelegate
		fakeWorker                *workerfakes.FakeWorker

		customTypes atc.VersionedResourceTypes
		privileged  bool

		fetchedVolume         worker.Volume
		fetchedMetadataReader io.ReadCloser
		fetchedVersion        atc.Version
		fetchErr              error
		teamID                int
	)
	BeforeEach(func() {
		fakeResourceFetcher = new(workerfakes.FakeFetcher)
		fakeResourceFactory = new(resourcefakes.FakeResourceFactory)
		fakeGetResource = new(resourcefakes.FakeResource)
		fakeResourceConfigFactory = new(dbfakes.FakeResourceConfigFactory)
		fakeCreatingContainer = new(dbfakes.FakeCreatingContainer)
		stderrBuf = gbytes.NewBuffer()

		logger = lagertest.NewTestLogger("test")
		imageResource = worker.ImageResource{
			Type:   "docker",
			Source: atc.Source{"some": "super-secret-sauce"},
			Params: atc.Params{"some": "params"},
		}
		version = nil
		ctx = context.Background()
		fakeImageFetchingDelegate = new(workerfakes.FakeImageFetchingDelegate)
		fakeImageFetchingDelegate.StderrReturns(stderrBuf)
		fakeWorker = new(workerfakes.FakeWorker)
		fakeWorker.NameReturns("some-worker")
		fakeWorker.TagsReturns(atc.Tags{"worker", "tags"})
		teamID = 123

		customTypes = atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Name:   "custom-type-a",
					Type:   "base-type",
					Source: atc.Source{"some": "a-source-param"},
				},
				Version: atc.Version{"some": "a-version"},
			},
			{
				ResourceType: atc.ResourceType{
					Name:   "custom-type-b",
					Type:   "custom-type-a",
					Source: atc.Source{"some": "b-source-param"},
				},
				Version: atc.Version{"some": "b-version"},
			},
		}

		fakeResourceCacheFactory = new(dbfakes.FakeResourceCacheFactory)

		fakeCheckResource = new(resourcefakes.FakeResource)

	})

	ExpectVersionSaveToDatabaseFails := func(expectedError error) {
		It("returns the error", func() {
			Expect(fetchErr).To(Equal(expectedError))
		})

		It("does not construct the 'get' resource", func() {
			Expect(fakeResourceFetcher.FetchCallCount()).To(Equal(0))
		})
	}

	JustBeforeEach(func() {
		imageResourceFetcher = image.NewImageResourceFetcherFactory(
			fakeResourceFactory,
			fakeResourceCacheFactory,
			fakeResourceConfigFactory,
			fakeResourceFetcher,
		).NewImageResourceFetcher(
			fakeWorker,
			imageResource,
			version,
			teamID,
			customTypes,
			fakeImageFetchingDelegate,
		)

		fetchedVolume, fetchedMetadataReader, fetchedVersion, fetchErr = imageResourceFetcher.Fetch(
			ctx,
			logger,
			fakeCreatingContainer,
			privileged,
		)
	})

	Context("when no version is specified", func() {
		BeforeEach(func() {
			version = nil
		})

		Context("when initializing the Check resource works", func() {
			var (
				fakeContainer *workerfakes.FakeContainer
			)

			BeforeEach(func() {
				fakeContainer = new(workerfakes.FakeContainer)
				fakeContainer.HandleReturns("some-handle")
				fakeWorker.FindOrCreateContainerReturnsOnCall(0, fakeContainer, nil)

			})

			Context("when the resource type the resource depends on a custom type", func() {
				var (
					fakeCheckResourceType  *resourcefakes.FakeResource
					customResourceTypeName = "custom-type-a"
				)

				BeforeEach(func() {
					imageResource = worker.ImageResource{
						Type:   customResourceTypeName,
						Source: atc.Source{"some": "source-param"},
						Params: atc.Params{"some": "params"},
					}

				})

				Context("and the custom type has a version", func() {
					BeforeEach(func() {
						fakeResourceFactory.NewResourceReturnsOnCall(0, fakeCheckResource)
						fakeResourceFactory.NewResourceReturnsOnCall(1, fakeGetResource)
					})
					It("does not check for versions of the custom type", func() {
						Expect(fakeWorker.FindOrCreateContainerCallCount()).To(Equal(1))
					})

					It("ran 'check' with the right config", func() {
						Expect(fakeCheckResource.CheckCallCount()).To(Equal(1))
						_, checkProcessSpec, checkImageContainer := fakeCheckResource.CheckArgsForCall(0)
						Expect(checkProcessSpec).To(Equal(runtime.ProcessSpec{
							Path: "/opt/resource/check",
						}))
						Expect(checkImageContainer).To(Equal(fakeContainer))
					})

				})

				Context("and the custom type does not have a version", func() {
					BeforeEach(func() {
						customTypes = atc.VersionedResourceTypes{
							{
								ResourceType: atc.ResourceType{
									Name:   "custom-type-a",
									Type:   "base-type",
									Source: atc.Source{"some": "param"},
								},
								Version: nil,
							},
						}

						fakeCheckResourceType = new(resourcefakes.FakeResource)
						fakeCheckResourceType.CheckReturns(
							[]atc.Version{{"some-key": "some-value"}},
							nil)
						fakeResourceFactory.NewResourceReturnsOnCall(0, fakeCheckResourceType)
						fakeResourceFactory.NewResourceReturnsOnCall(1, fakeCheckResource)
						fakeResourceFactory.NewResourceReturnsOnCall(2, fakeGetResource)

						fakeWorker.FindOrCreateContainerReturns(fakeContainer, nil)
					})

					It("checks for the latest version of the resource type", func() {
						By("find or create a resource container")
						_, _, _, _, _, containerSpec, _ := fakeWorker.FindOrCreateContainerArgsForCall(0)
						Expect(containerSpec.ImageSpec.ResourceType).To(Equal("custom-type-a"))

						By("calling the resource type's check script")
						Expect(fakeCheckResourceType.CheckCallCount()).To(Equal(1))
					})

					Context("when a version of the custom resource type is found", func() {
						BeforeEach(func() {
							fakeCheckResourceType.CheckReturns([]atc.Version{{"some": "version"}}, nil)
						})

						It("uses the version of the custom type when checking for the original resource", func() {
							Expect(fakeWorker.FindOrCreateContainerCallCount()).To(Equal(2))
							_, _, _, _, _, containerSpec, customTypes := fakeWorker.FindOrCreateContainerArgsForCall(1)
							Expect(containerSpec.ImageSpec.ResourceType).To(Equal("custom-type-a"))
							Expect(customTypes[0].Version).To(Equal(atc.Version{"some": "version"}))
						})
					})
				})
			})

			Context("when check returns a version", func() {
				BeforeEach(func() {
					fakeResourceFactory.NewResourceReturnsOnCall(0, fakeCheckResource)
					fakeResourceFactory.NewResourceReturnsOnCall(1, fakeGetResource)

					fakeCheckResource.CheckReturns([]atc.Version{{"v": "1"}}, nil)
				})

				Context("when saving the version in the database succeeds", func() {
					BeforeEach(func() {
						fakeImageFetchingDelegate.ImageVersionDeterminedReturns(nil)
					})

					Context("when fetching resource fails", func() {
						var someError error
						BeforeEach(func() {
							someError = errors.New("some thing bad happened")
							fakeResourceFetcher.FetchReturns(worker.GetResult{}, &workerfakes.FakeVolume{}, someError)
						})

						It("returns error", func() {
							Expect(fetchErr).To(Equal(someError))
						})
					})

					Context("when fetching resource succeeds", func() {
						var (
							fakeUsedResourceCache *dbfakes.FakeUsedResourceCache
							fakeVolume            *workerfakes.FakeVolume
						)

						BeforeEach(func() {
							fakeVolume = &workerfakes.FakeVolume{}
							fakeResourceFetcher.FetchReturns(worker.GetResult{}, fakeVolume, nil)

							fakeVolume.StreamOutReturns(tgzStreamWith("some-tar-contents"), nil)

							fakeUsedResourceCache = new(dbfakes.FakeUsedResourceCache)
							fakeResourceCacheFactory.FindOrCreateResourceCacheReturns(fakeUsedResourceCache, nil)
						})

						Context("when the resource has a volume", func() {
							var (
								volumePath            string
								fakeUsedResourceCache *dbfakes.FakeUsedResourceCache
								someStdoutWriter      io.Writer
								someStderrWriter      io.Writer
							)

							BeforeEach(func() {
								fakeUsedResourceCache = new(dbfakes.FakeUsedResourceCache)

								volumePath = "C:/Documents and Settings/Evan/My Documents"
								fakeVolume.PathReturns(volumePath)

								someStdoutWriter = gbytes.NewBuffer()
								someStderrWriter = gbytes.NewBuffer()

								fakeImageFetchingDelegate.StdoutReturns(someStdoutWriter)
								fakeImageFetchingDelegate.StderrReturns(someStderrWriter)

								fakeResourceFactory.NewResourceReturns(fakeGetResource)

								privileged = true
							})

							It("calls resourceFetcher.Fetch with the correct args", func() {
								actualCtx, _, actualMetadata, actualWorker,
									actualContainerSpec, actualProcessSpec, actualResource,
									actualContainerOwner, actualImageFetcherSpec,
									actualResourceCache, lockname := fakeResourceFetcher.FetchArgsForCall(0)

								Expect(actualCtx).To(Equal(ctx))
								Expect(actualMetadata).To(Equal(db.ContainerMetadata{
									Type: db.ContainerTypeGet,
								}))
								Expect(actualWorker).To(Equal(fakeWorker))
								Expect(actualContainerSpec.ImageSpec).To(Equal(worker.ImageSpec{
									ResourceType: "docker",
								}))
								Expect(actualContainerSpec.TeamID).To(Equal(123))
								Expect(actualProcessSpec).To(Equal(runtime.ProcessSpec{
									Path:         "/opt/resource/in",
									Args:         []string{resource.ResourcesDir("get")},
									StdoutWriter: someStdoutWriter,
									StderrWriter: someStderrWriter,
								}))

								Expect(actualResource).To(Equal(fakeGetResource))
								Expect(actualContainerOwner).To(Equal(db.NewImageGetContainerOwner(fakeCreatingContainer, 123)))
								Expect(actualImageFetcherSpec).To(Equal(worker.ImageFetcherSpec{
									customTypes,
									fakeImageFetchingDelegate,
								}))
								Expect(actualResourceCache).To(Equal(fakeUsedResourceCache))
								Expect(lockname).To(Equal("18c3de3f8ea112ba52e01f279b6cc62335b4bec2f359b9be7636a5ad7bf98f8c"))
							})

							It("succeeds", func() {
								Expect(fetchErr).To(BeNil())
							})

							It("returns the image volume", func() {
								Expect(fetchedVolume).To(Equal(fakeVolume))
							})

							It("calls StreamOut on the versioned source with the right metadata path", func() {
								Expect(fakeVolume.StreamOutCallCount()).To(Equal(1))
								volumeCtx, metadataFilePath := fakeVolume.StreamOutArgsForCall(0)
								Expect(volumeCtx).To(Equal(ctx))
								Expect(metadataFilePath).To(Equal("metadata.json"))
							})

							It("returns a tar stream containing the contents of metadata.json", func() {
								Expect(ioutil.ReadAll(fetchedMetadataReader)).To(Equal([]byte("some-tar-contents")))
							})

							It("has the version on the image", func() {
								Expect(fetchedVersion).To(Equal(atc.Version{"v": "1"}))
							})

							It("saved the image resource version in the database", func() {
								Expect(fakeImageFetchingDelegate.ImageVersionDeterminedCallCount()).To(Equal(1))
								Expect(fakeImageFetchingDelegate.ImageVersionDeterminedArgsForCall(0)).To(Equal(fakeUsedResourceCache))
							})

							Context("when streaming the metadata out fails", func() {
								disaster := errors.New("nope")

								BeforeEach(func() {
									fakeVolume.StreamOutReturns(nil, disaster)
								})

								It("returns the error", func() {
									Expect(fetchErr).To(Equal(disaster))
								})
							})

							Context("when the resource still does not have a volume for some reason", func() {
								BeforeEach(func() {
									fakeResourceFetcher.FetchReturns(worker.GetResult{}, nil, nil)
								})

								It("returns an appropriate error", func() {
									Expect(fetchErr).To(Equal(image.ErrImageGetDidNotProduceVolume))
								})
							})

							//This is the only test thats not repeated in the flow below
							It("ran 'check' with the right config", func() {
								Expect(fakeCheckResource.CheckCallCount()).To(Equal(1))
								_, checkProcessSpec, checkImageContainer := fakeCheckResource.CheckArgsForCall(0)
								Expect(checkProcessSpec).To(Equal(runtime.ProcessSpec{
									Path: "/opt/resource/check",
								}))
								Expect(checkImageContainer).To(Equal(fakeContainer))
							})
						})
					})
				})

				Context("when saving the version in the database fails", func() {
					var imageVersionSavingCalamity = errors.New("hang in there bud")
					BeforeEach(func() {
						fakeImageFetchingDelegate.ImageVersionDeterminedReturns(imageVersionSavingCalamity)
					})

					ExpectVersionSaveToDatabaseFails(imageVersionSavingCalamity)
				})
			})

			Context("when check returns no versions", func() {
				BeforeEach(func() {
					fakeCheckResource.CheckReturns([]atc.Version{}, nil)
					fakeResourceFactory.NewResourceReturnsOnCall(0, fakeCheckResource)
				})

				It("exits with ErrImageUnavailable", func() {
					Expect(fetchErr).To(Equal(image.ErrImageUnavailable))
				})

				It("does not attempt to save any versions in the database", func() {
					Expect(fakeImageFetchingDelegate.ImageVersionDeterminedCallCount()).To(Equal(0))
				})
			})

			Context("when check returns an error", func() {
				var (
					disaster error
				)

				BeforeEach(func() {
					disaster = errors.New("wah")
					fakeCheckResource.CheckReturns(nil, disaster)
					fakeResourceFactory.NewResourceReturnsOnCall(0, fakeCheckResource)
				})

				It("returns the error", func() {
					Expect(fetchErr).To(Equal(disaster))
				})

				It("does not construct the 'get' resource", func() {
					Expect(fakeResourceFetcher.FetchCallCount()).To(Equal(0))
				})
			})
		})

		Context("when creating or finding the Check container fails", func() {
			var (
				disaster error
			)

			BeforeEach(func() {
				disaster = errors.New("wah")
				fakeWorker.FindOrCreateContainerReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(fetchErr).To(Equal(disaster))
			})

			It("does not construct the 'get' resource", func() {
				Expect(fakeResourceFetcher.FetchCallCount()).To(Equal(0))
			})
		})
	})

	Context("when a version is specified", func() {
		BeforeEach(func() {
			version = atc.Version{"v": "1"}
			fakeResourceFactory.NewResourceReturnsOnCall(0, fakeGetResource)
		})

		Context("when saving the version in the database succeeds", func() {
			BeforeEach(func() {
				fakeImageFetchingDelegate.ImageVersionDeterminedReturns(nil)
			})

			Context("when fetching resource fails", func() {
				var someError error
				BeforeEach(func() {
					someError = errors.New("some thing bad happened")
					fakeResourceFetcher.FetchReturns(worker.GetResult{}, &workerfakes.FakeVolume{}, someError)
				})

				It("returns error", func() {
					Expect(fetchErr).To(Equal(someError))
				})
			})

			Context("when fetching resource succeeds", func() {
				var (
					fakeUsedResourceCache *dbfakes.FakeUsedResourceCache
					fakeVolume            *workerfakes.FakeVolume
				)

				BeforeEach(func() {
					fakeVolume = &workerfakes.FakeVolume{}
					fakeResourceFetcher.FetchReturns(worker.GetResult{}, fakeVolume, nil)

					fakeVolume.StreamOutReturns(tgzStreamWith("some-tar-contents"), nil)

					fakeUsedResourceCache = new(dbfakes.FakeUsedResourceCache)
					fakeResourceCacheFactory.FindOrCreateResourceCacheReturns(fakeUsedResourceCache, nil)
				})

				Context("when the resource has a volume", func() {
					var (
						volumePath            string
						fakeUsedResourceCache *dbfakes.FakeUsedResourceCache
						someStdoutWriter      io.Writer
						someStderrWriter      io.Writer
					)

					BeforeEach(func() {
						fakeUsedResourceCache = new(dbfakes.FakeUsedResourceCache)

						volumePath = "C:/Documents and Settings/Evan/My Documents"
						fakeVolume.PathReturns(volumePath)

						someStdoutWriter = gbytes.NewBuffer()
						someStderrWriter = gbytes.NewBuffer()

						fakeImageFetchingDelegate.StdoutReturns(someStdoutWriter)
						fakeImageFetchingDelegate.StderrReturns(someStderrWriter)

						fakeResourceFactory.NewResourceReturns(fakeGetResource)

						privileged = true
					})

					It("calls resourceFetcher.Fetch with the correct args", func() {
						actualCtx, _, actualMetadata, actualWorker,
							actualContainerSpec, actualProcessSpec, actualResource,
							actualContainerOwner, actualImageFetcherSpec,
							actualResourceCache, lockname := fakeResourceFetcher.FetchArgsForCall(0)

						Expect(actualCtx).To(Equal(ctx))
						Expect(actualMetadata).To(Equal(db.ContainerMetadata{
							Type: db.ContainerTypeGet,
						}))
						Expect(actualWorker).To(Equal(fakeWorker))
						Expect(actualContainerSpec.ImageSpec).To(Equal(worker.ImageSpec{
							ResourceType: "docker",
						}))
						Expect(actualContainerSpec.TeamID).To(Equal(123))
						Expect(actualProcessSpec).To(Equal(runtime.ProcessSpec{
							Path:         "/opt/resource/in",
							Args:         []string{resource.ResourcesDir("get")},
							StdoutWriter: someStdoutWriter,
							StderrWriter: someStderrWriter,
						}))

						Expect(actualResource).To(Equal(fakeGetResource))
						Expect(actualContainerOwner).To(Equal(db.NewImageGetContainerOwner(fakeCreatingContainer, 123)))
						Expect(actualImageFetcherSpec).To(Equal(worker.ImageFetcherSpec{
							customTypes,
							fakeImageFetchingDelegate,
						}))
						Expect(actualResourceCache).To(Equal(fakeUsedResourceCache))
						Expect(lockname).To(Equal("18c3de3f8ea112ba52e01f279b6cc62335b4bec2f359b9be7636a5ad7bf98f8c"))
					})

					It("succeeds", func() {
						Expect(fetchErr).To(BeNil())
					})

					It("returns the image volume", func() {
						Expect(fetchedVolume).To(Equal(fakeVolume))
					})

					It("calls StreamOut on the versioned source with the right metadata path", func() {
						Expect(fakeVolume.StreamOutCallCount()).To(Equal(1))
						volumeCtx, metadataFilePath := fakeVolume.StreamOutArgsForCall(0)
						Expect(volumeCtx).To(Equal(ctx))
						Expect(metadataFilePath).To(Equal("metadata.json"))
					})

					It("returns a tar stream containing the contents of metadata.json", func() {
						Expect(ioutil.ReadAll(fetchedMetadataReader)).To(Equal([]byte("some-tar-contents")))
					})

					It("has the version on the image", func() {
						Expect(fetchedVersion).To(Equal(atc.Version{"v": "1"}))
					})

					It("saved the image resource version in the database", func() {
						Expect(fakeImageFetchingDelegate.ImageVersionDeterminedCallCount()).To(Equal(1))
						Expect(fakeImageFetchingDelegate.ImageVersionDeterminedArgsForCall(0)).To(Equal(fakeUsedResourceCache))
					})

					Context("when streaming the metadata out fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeVolume.StreamOutReturns(nil, disaster)
						})

						It("returns the error", func() {
							Expect(fetchErr).To(Equal(disaster))
						})
					})

					Context("when the resource still does not have a volume for some reason", func() {
						BeforeEach(func() {
							fakeResourceFetcher.FetchReturns(worker.GetResult{}, nil, nil)
						})

						It("returns an appropriate error", func() {
							Expect(fetchErr).To(Equal(image.ErrImageGetDidNotProduceVolume))
						})
					})
				})
			})
		})

		Context("when saving the version in the database fails", func() {
			var imageVersionSavingCalamity = errors.New("hang in there bud")
			BeforeEach(func() {
				fakeImageFetchingDelegate.ImageVersionDeterminedReturns(imageVersionSavingCalamity)
			})

			ExpectVersionSaveToDatabaseFails(imageVersionSavingCalamity)
		})
	})
})

func tgzStreamWith(metadata string) io.ReadCloser {
	buffer := gbytes.NewBuffer()

	zstdWriter := zstd.NewWriter(buffer)
	tarWriter := tar.NewWriter(zstdWriter)

	err := tarWriter.WriteHeader(&tar.Header{
		Name: "metadata.json",
		Mode: 0600,
		Size: int64(len(metadata)),
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = tarWriter.Write([]byte(metadata))
	Expect(err).NotTo(HaveOccurred())

	err = tarWriter.Close()
	Expect(err).NotTo(HaveOccurred())

	err = zstdWriter.Close()
	Expect(err).NotTo(HaveOccurred())

	return buffer
}
