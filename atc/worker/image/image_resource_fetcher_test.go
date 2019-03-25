package image_test

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/resource/resourcefakes"
	v2 "github.com/concourse/concourse/atc/resource/v2"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/image"
	"github.com/concourse/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Image", func() {
	var fakeResourceFactory *resourcefakes.FakeResourceFactory
	var fakeResourceFetcher *resourcefakes.FakeFetcher
	var fakeResourceCacheFactory *dbfakes.FakeResourceCacheFactory
	var fakeCreatingContainer *dbfakes.FakeCreatingContainer
	var fakeCheckResource *resourcefakes.FakeResource
	var fakeCheckResourceType *resourcefakes.FakeResource

	var imageResourceFetcher image.ImageResourceFetcher

	var stderrBuf *gbytes.Buffer

	var logger lager.Logger
	var imageResource worker.ImageResource
	var version atc.Version
	var defaultSpace atc.Space
	var ctx context.Context
	var fakeImageFetchingDelegate *workerfakes.FakeImageFetchingDelegate
	var fakeWorker *workerfakes.FakeWorker

	var customTypes creds.VersionedResourceTypes
	var privileged bool

	var fetchedVolume worker.Volume
	var fetchedMetadataReader io.ReadCloser
	var fetchedVersion atc.Version
	var fetchErr error
	var teamID int
	var variables template.StaticVariables

	var resourceDefaultSpace atc.Space
	var resourceLatestVersions map[atc.Space]atc.Version
	var resourceTypeDefaultSpace atc.Space
	var resourceTypeLatestVersions map[atc.Space]atc.Version

	var resourceCheckError error

	BeforeEach(func() {
		fakeResourceFactory = new(resourcefakes.FakeResourceFactory)
		fakeResourceFetcher = new(resourcefakes.FakeFetcher)
		fakeCreatingContainer = new(dbfakes.FakeCreatingContainer)
		fakeCheckResource = new(resourcefakes.FakeResource)
		fakeCheckResourceType = new(resourcefakes.FakeResource)

		stderrBuf = gbytes.NewBuffer()

		variables = template.StaticVariables{
			"source-param":   "super-secret-sauce",
			"a-source-param": "super-secret-a-source",
			"b-source-param": "super-secret-b-source",
		}

		logger = lagertest.NewTestLogger("test")
		imageResource = worker.ImageResource{
			Type:   "docker",
			Source: creds.NewSource(variables, atc.Source{"some": "((source-param))"}),
			Params: &atc.Params{"some": "params"},
		}
		ctx = context.Background()
		fakeImageFetchingDelegate = new(workerfakes.FakeImageFetchingDelegate)
		fakeImageFetchingDelegate.StderrReturns(stderrBuf)
		fakeWorker = new(workerfakes.FakeWorker)
		fakeWorker.NameReturns("some-worker")
		fakeWorker.TagsReturns(atc.Tags{"worker", "tags"})
		teamID = 123

		customTypes = creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Name:   "custom-type-a",
					Type:   "base-type",
					Source: atc.Source{"some": "((a-source-param))"},
				},
				Version: atc.Version{"some": "a-version"},
			},
			{
				ResourceType: atc.ResourceType{
					Name:   "custom-type-b",
					Type:   "custom-type-a",
					Source: atc.Source{"some": "((b-source-param))"},
				},
				Version: atc.Version{"some": "b-version"},
			},
		})

		fakeResourceCacheFactory = new(dbfakes.FakeResourceCacheFactory)
		fakeResourceFactory.NewResourceForContainerReturns(fakeCheckResource, nil)

		version = nil
		defaultSpace = ""
		resourceDefaultSpace = ""
		resourceTypeDefaultSpace = ""
		resourceLatestVersions = nil
		resourceTypeLatestVersions = nil
		resourceCheckError = nil
	})

	JustBeforeEach(func() {
		imageResourceFetcher = image.NewImageResourceFetcherFactory(
			fakeResourceCacheFactory,
			fakeResourceFetcher,
			fakeResourceFactory,
		).NewImageResourceFetcher(
			fakeWorker,
			imageResource,
			version,
			defaultSpace,
			teamID,
			customTypes,
			fakeImageFetchingDelegate,
		)

		fakeCheckResourceType.CheckStub = func(context context.Context, handler v2.CheckEventHandler, source atc.Source, from map[atc.Space]atc.Version) error {
			handler.(*image.CheckEventHandler).SavedDefaultSpace = resourceTypeDefaultSpace
			handler.(*image.CheckEventHandler).SavedLatestVersions = resourceTypeLatestVersions
			return nil
		}

		fakeCheckResource.CheckStub = func(context context.Context, handler v2.CheckEventHandler, source atc.Source, from map[atc.Space]atc.Version) error {
			handler.(*image.CheckEventHandler).SavedDefaultSpace = resourceDefaultSpace
			handler.(*image.CheckEventHandler).SavedLatestVersions = resourceLatestVersions
			return resourceCheckError
		}

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

				fakeResourceFactory.NewResourceForContainerReturnsOnCall(0, fakeCheckResource, nil)
			})

			Context("when the resource type the resource depends on a custom type", func() {
				var (
					customResourceTypeName = "custom-type-a"
				)

				BeforeEach(func() {
					imageResource = worker.ImageResource{
						Type:   customResourceTypeName,
						Source: creds.NewSource(variables, atc.Source{"some": "((source-param))"}),
						Params: &atc.Params{"some": "params"},
					}

				})

				Context("and the custom type has a version", func() {
					It("does not check for versions of the custom type", func() {
						Expect(fakeWorker.FindOrCreateContainerCallCount()).To(Equal(1))
					})
				})

				Context("and the custom type does not have a version", func() {
					BeforeEach(func() {
						customTypes = creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
							{
								ResourceType: atc.ResourceType{
									Name:   "custom-type-a",
									Type:   "base-type",
									Source: atc.Source{"some": "param"},
								},
								Version: nil,
							},
						})

						fakeCheckResourceType = new(resourcefakes.FakeResource)
						fakeWorker.FindOrCreateContainerReturns(fakeContainer, nil)
						fakeResourceFactory.NewResourceForContainerReturnsOnCall(0, fakeCheckResourceType, nil)

						fakeResourceFactory.NewResourceForContainerReturnsOnCall(1, fakeCheckResource, nil)
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
							resourceTypeDefaultSpace = "space"
							resourceTypeLatestVersions = map[atc.Space]atc.Version{resourceTypeDefaultSpace: {"some": "version"}}
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
					resourceDefaultSpace = "space"
					resourceLatestVersions = map[atc.Space]atc.Version{resourceDefaultSpace: {"v": "1"}}
				})

				Context("when saving the version in the database succeeds", func() {
					BeforeEach(func() {
						fakeImageFetchingDelegate.ImageVersionDeterminedReturns(nil)
					})

					Context("when fetching resource fails", func() {
						BeforeEach(func() {
							fakeResourceFetcher.FetchReturns(nil, resource.ErrInterrupted)
						})

						It("returns error", func() {
							Expect(fetchErr).To(Equal(resource.ErrInterrupted))
						})
					})

					Context("when fetching resource succeeds", func() {
						var (
							fakeVolume            *workerfakes.FakeVolume
							fakeUsedResourceCache *dbfakes.FakeUsedResourceCache
						)

						BeforeEach(func() {
							fakeVolume = new(workerfakes.FakeVolume)
							fakeVolume.StreamOutReturns(tgzStreamWith("some-tar-contents"), nil)
							fakeResourceFetcher.FetchReturns(fakeVolume, nil)

							fakeUsedResourceCache = new(dbfakes.FakeUsedResourceCache)
							fakeResourceCacheFactory.FindOrCreateResourceCacheReturns(fakeUsedResourceCache, nil)
						})

						Context("when the resource has a volume", func() {
							var (
								volumePath            string
								fakeUsedResourceCache *dbfakes.FakeUsedResourceCache
							)

							BeforeEach(func() {
								fakeUsedResourceCache = new(dbfakes.FakeUsedResourceCache)
								volumePath = "C:/Documents and Settings/Evan/My Documents"

								fakeVolume.PathReturns(volumePath)

								privileged = true
							})

							Context("calling NewResourceForContainer", func() {
								BeforeEach(func() {
									fakeWorker.FindOrCreateContainerReturns(fakeContainer, nil)
									fakeResourceFactory.NewResourceForContainerReturns(fakeCheckResource, nil)
								})

								It("created the 'check' resource with the correct session, with the currently fetching type removed from the set", func() {
									Expect(fakeWorker.FindOrCreateContainerCallCount()).To(Equal(1))
									cctx, _, delegate, owner, metadata, containerSpec, actualCustomTypes := fakeWorker.FindOrCreateContainerArgsForCall(0)
									Expect(cctx).To(Equal(ctx))
									Expect(owner).To(Equal(db.NewImageCheckContainerOwner(fakeCreatingContainer, 123)))
									Expect(metadata).To(Equal(db.ContainerMetadata{
										Type: db.ContainerTypeCheck,
									}))
									Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
										ResourceType: "docker",
									}))
									Expect(containerSpec.TeamID).To(Equal(123))
									Expect(actualCustomTypes).To(Equal(customTypes))
									Expect(delegate).To(Equal(fakeImageFetchingDelegate))

									Expect(fakeResourceFactory.NewResourceForContainerCallCount()).To(Equal(1))
									_, container := fakeResourceFactory.NewResourceForContainerArgsForCall(0)
									Expect(container.Handle()).To(Equal("some-handle"))
								})
							})

							It("succeeds", func() {
								Expect(fetchErr).To(BeNil())
							})

							It("returns the image volume", func() {
								Expect(fetchedVolume).To(Equal(fakeVolume))
							})

							It("calls StreamOut on the versioned source with the right metadata path", func() {
								Expect(fakeVolume.StreamOutCallCount()).To(Equal(1))
								Expect(fakeVolume.StreamOutArgsForCall(0)).To(Equal("metadata.json"))
							})

							It("returns a tar stream containing the contents of metadata.json", func() {
								Expect(ioutil.ReadAll(fetchedMetadataReader)).To(Equal([]byte("some-tar-contents")))
							})

							It("has the version on the image", func() {
								Expect(fetchedVersion).To(Equal(atc.Version{"v": "1"}))
							})

							It("created the 'check' resource with the correct session, with the currently fetching type removed from the set", func() {
								Expect(fakeWorker.FindOrCreateContainerCallCount()).To(Equal(1))
								cctx, _, delegate, owner, metadata, containerSpec, actualCustomTypes := fakeWorker.FindOrCreateContainerArgsForCall(0)
								Expect(cctx).To(Equal(ctx))
								Expect(owner).To(Equal(db.NewImageCheckContainerOwner(fakeCreatingContainer, 123)))
								Expect(metadata).To(Equal(db.ContainerMetadata{
									Type: db.ContainerTypeCheck,
								}))
								Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
									ResourceType: "docker",
								}))
								Expect(containerSpec.TeamID).To(Equal(123))
								Expect(actualCustomTypes).To(Equal(customTypes))
								Expect(delegate).To(Equal(fakeImageFetchingDelegate))

								Expect(fakeResourceFactory.NewResourceForContainerCallCount()).To(Equal(1))
								_, container := fakeResourceFactory.NewResourceForContainerArgsForCall(0)
								Expect(container.Handle()).To(Equal("some-handle"))
							})

							It("ran 'check' with the right config", func() {
								Expect(fakeCheckResource.CheckCallCount()).To(Equal(1))
								_, _, checkSource, checkVersion := fakeCheckResource.CheckArgsForCall(0)
								Expect(checkVersion).To(BeNil())
								Expect(checkSource).To(Equal(atc.Source{"some": "super-secret-sauce"}))
							})

							It("saved the image resource version in the database", func() {
								Expect(fakeImageFetchingDelegate.ImageVersionDeterminedCallCount()).To(Equal(1))
								Expect(fakeImageFetchingDelegate.ImageVersionDeterminedArgsForCall(0)).To(Equal(fakeUsedResourceCache))
							})

							It("fetches resource with correct session", func() {
								Expect(fakeResourceFetcher.FetchCallCount()).To(Equal(1))
								_, _, session, eventHandler, actualWorker, containerSpec, actualCustomTypes, resourceInstance, delegate := fakeResourceFetcher.FetchArgsForCall(0)
								Expect(session).To(Equal(resource.Session{
									Metadata: db.ContainerMetadata{
										Type: db.ContainerTypeGet,
									},
								}))
								Expect(eventHandler).To(Equal(image.NewGetEventHandler()))
								Expect(actualWorker.Name()).To(Equal("some-worker"))
								Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
									ResourceType: "docker",
								}))
								Expect(containerSpec.TeamID).To(Equal(123))
								Expect(resourceInstance).To(Equal(resource.NewResourceInstance(
									"docker",
									atc.Space("space"),
									atc.Version{"v": "1"},
									atc.Source{"some": "super-secret-sauce"},
									atc.Params{"some": "params"},
									customTypes,
									fakeUsedResourceCache,
									db.NewImageGetContainerOwner(fakeCreatingContainer, teamID),
								)))
								Expect(actualCustomTypes).To(Equal(customTypes))
								Expect(delegate).To(Equal(fakeImageFetchingDelegate))
								expectedLockName := fmt.Sprintf("%x",
									sha256.Sum256([]byte(
										`{"type":"docker","space":"space","version":{"v":"1"},"source":{"some":"super-secret-sauce"},"params":{"some":"params"},"worker_name":"fake-worker-name"}`,
									)),
								)
								Expect(resourceInstance.LockName("fake-worker-name")).To(Equal(expectedLockName))
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

							Context("when a volume is not returned from the fetch", func() {
								BeforeEach(func() {
									fakeResourceFetcher.FetchReturns(nil, nil)
								})

								It("returns an appropriate error", func() {
									Expect(fetchErr).To(Equal(image.ErrImageGetDidNotProduceVolume))
								})
							})
						})
					})
				})

				Context("when saving the version in the database fails", func() {
					var imageVersionSavingCalamity error
					BeforeEach(func() {
						imageVersionSavingCalamity = errors.New("hang in there bud")
						fakeImageFetchingDelegate.ImageVersionDeterminedReturns(imageVersionSavingCalamity)
					})

					It("returns the error", func() {
						Expect(fetchErr).To(Equal(imageVersionSavingCalamity))
					})

					It("does not construct the 'get' resource", func() {
						Expect(fakeResourceFetcher.FetchCallCount()).To(Equal(0))
					})
				})
			})

			Context("when the default space is not specified", func() {
				Context("when the default space is not returned by the check", func() {
					BeforeEach(func() {
						resourceDefaultSpace = ""
					})

					It("exists with ErrNoSpaceSpecified", func() {
						Expect(fetchErr).To(Equal(image.ErrNoSpaceSpecified))
					})
				})

				Context("when the default space is returned by the check", func() {
					BeforeEach(func() {
						resourceDefaultSpace = "space"
					})

					It("uses the default space and version returned by check", func() {
						// XXX: have find or create resource cache args for call use the default space and version
					})
				})
			})

			Context("when the default space is specified", func() {
				BeforeEach(func() {
					defaultSpace = "space"
				})

				It("uses the default space", func() {
					// XXX: have find or create resource cache args for call use the default space
				})

				Context("when a default space is returned by the check", func() {
					BeforeEach(func() {
						resourceDefaultSpace = "check-space"
					})

					It("uses the default space specified by the user", func() {
						// XXX: have find or create resource cache args for call use the default space "space"
					})
				})
			})

			Context("when check returns no versions", func() {
				BeforeEach(func() {
					resourceDefaultSpace = "space"
					resourceLatestVersions = map[atc.Space]atc.Version{}
				})

				It("exits with ErrImageUnavailable", func() {
					Expect(fetchErr).To(Equal(image.ErrImageUnavailable))
				})

				It("does not attempt to save any versions in the database", func() {
					Expect(fakeImageFetchingDelegate.ImageVersionDeterminedCallCount()).To(Equal(0))
				})
			})

			Context("when check returns an error", func() {
				disaster := errors.New("wah")

				BeforeEach(func() {
					resourceCheckError = disaster
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

	Context("when a version and space is specified", func() {
		BeforeEach(func() {
			version = atc.Version{"some": "version"}
			defaultSpace = atc.Space("space")
		})

		Context("when saving the version in the database succeeds", func() {
			BeforeEach(func() {
				fakeImageFetchingDelegate.ImageVersionDeterminedReturns(nil)
			})

			Context("when fetching resource fails", func() {
				BeforeEach(func() {
					fakeResourceFetcher.FetchReturns(nil, resource.ErrInterrupted)
				})

				It("returns error", func() {
					Expect(fetchErr).To(Equal(resource.ErrInterrupted))
				})
			})

			Context("when fetching resource succeeds", func() {
				var (
					fakeUsedResourceCache *dbfakes.FakeUsedResourceCache
					fakeVolume            *workerfakes.FakeVolume
				)

				BeforeEach(func() {
					fakeVolume = new(workerfakes.FakeVolume)
					fakeVolume.StreamOutReturns(tgzStreamWith("some-tar-contents"), nil)
					fakeResourceFetcher.FetchReturns(fakeVolume, nil)

					fakeUsedResourceCache = new(dbfakes.FakeUsedResourceCache)
					fakeResourceCacheFactory.FindOrCreateResourceCacheReturns(fakeUsedResourceCache, nil)
				})

				Context("when the resource has a volume", func() {
					var (
						volumePath string
					)

					BeforeEach(func() {
						volumePath = "C:/Documents and Settings/Evan/My Documents"

						fakeVolume.PathReturns(volumePath)

						privileged = true
					})

					It("does not construct a new resource for checking", func() {
						Expect(fakeWorker.FindOrCreateContainerCallCount()).To(BeZero())
						Expect(fakeResourceFactory.NewResourceForContainerCallCount()).To(BeZero())
					})

					It("succeeds", func() {
						Expect(fetchErr).To(BeNil())
					})

					It("returns the image volume", func() {
						Expect(fetchedVolume).To(Equal(fakeVolume))
					})

					It("calls StreamOut on the versioned source with the right metadata path", func() {
						Expect(fakeVolume.StreamOutCallCount()).To(Equal(1))
						Expect(fakeVolume.StreamOutArgsForCall(0)).To(Equal("metadata.json"))
					})

					It("returns a tar stream containing the contents of metadata.json", func() {
						Expect(ioutil.ReadAll(fetchedMetadataReader)).To(Equal([]byte("some-tar-contents")))
					})

					It("has the version on the image", func() {
						Expect(fetchedVersion).To(Equal(atc.Version{"some": "version"}))
					})

					It("saved the image resource version in the database", func() {
						Expect(fakeImageFetchingDelegate.ImageVersionDeterminedCallCount()).To(Equal(1))
						Expect(fakeImageFetchingDelegate.ImageVersionDeterminedArgsForCall(0)).To(Equal(fakeUsedResourceCache))
					})

					It("fetches resource with correct session", func() {
						Expect(fakeResourceFetcher.FetchCallCount()).To(Equal(1))
						_, _, session, eventHandler, actualWorker, containerSpec, actualCustomTypes, resourceInstance, delegate := fakeResourceFetcher.FetchArgsForCall(0)
						Expect(session).To(Equal(resource.Session{
							Metadata: db.ContainerMetadata{
								Type: db.ContainerTypeGet,
							},
						}))
						Expect(eventHandler).To(Equal(image.NewGetEventHandler()))
						Expect(actualWorker.Name()).To(Equal("some-worker"))
						Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
							ResourceType: "docker",
						}))
						Expect(containerSpec.TeamID).To(Equal(teamID))
						Expect(resourceInstance).To(Equal(resource.NewResourceInstance(
							"docker",
							atc.Space("space"),
							atc.Version{"some": "version"},
							atc.Source{"some": "super-secret-sauce"},
							atc.Params{"some": "params"},
							customTypes,
							fakeUsedResourceCache,
							db.NewImageGetContainerOwner(fakeCreatingContainer, teamID),
						)))
						Expect(actualCustomTypes).To(Equal(customTypes))
						Expect(delegate).To(Equal(fakeImageFetchingDelegate))
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
				})
			})
		})

		Context("when saving the version in the database fails", func() {
			var imageVersionSavingCalamity error
			BeforeEach(func() {
				imageVersionSavingCalamity = errors.New("hang in there bud")
				fakeImageFetchingDelegate.ImageVersionDeterminedReturns(imageVersionSavingCalamity)
			})

			It("returns the error", func() {
				Expect(fetchErr).To(Equal(imageVersionSavingCalamity))
			})

			It("does not construct the 'get' resource", func() {
				Expect(fakeResourceFetcher.FetchCallCount()).To(Equal(0))
			})
		})
	})
})

func tgzStreamWith(metadata string) io.ReadCloser {
	buffer := gbytes.NewBuffer()

	gzWriter := gzip.NewWriter(buffer)
	tarWriter := tar.NewWriter(gzWriter)

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

	err = gzWriter.Close()
	Expect(err).NotTo(HaveOccurred())

	return buffer
}
