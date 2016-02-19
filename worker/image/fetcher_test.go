package image_test

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	rfakes "github.com/concourse/atc/resource/fakes"
	"github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/fakes"
	"github.com/concourse/atc/worker/image"
	"github.com/concourse/atc/worker/image/fakes"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Fetcher", func() {
	var fakeTrackerFactory *fakes.FakeTrackerFactory
	var fakeImageTracker *rfakes.FakeTracker

	var fetcher image.Fetcher

	var stderrBuf *gbytes.Buffer

	var logger lager.Logger
	var imageConfig atc.TaskImageConfig
	var signals chan os.Signal
	var identifier worker.Identifier
	var metadata worker.Metadata
	var fakeImageFetchingDelegate *wfakes.FakeImageFetchingDelegate
	var fakeWorker *wfakes.FakeClient
	var customTypes atc.ResourceTypes

	var fetchedImage worker.Image
	var fetchErr error

	BeforeEach(func() {
		fakeTrackerFactory = new(fakes.FakeTrackerFactory)

		fakeImageTracker = new(rfakes.FakeTracker)
		fakeTrackerFactory.TrackerForReturns(fakeImageTracker)

		fetcher = image.NewFetcher(fakeTrackerFactory)

		stderrBuf = gbytes.NewBuffer()

		logger = lagertest.NewTestLogger("test")
		imageConfig = atc.TaskImageConfig{
			Type:   "docker",
			Source: atc.Source{"some": "source"},
		}
		signals = make(chan os.Signal)
		identifier = worker.Identifier{
			BuildID: 1234,
			PlanID:  "some-plan-id",
		}
		metadata = worker.Metadata{
			PipelineName:         "some-pipeline",
			Type:                 db.ContainerTypeCheck,
			StepName:             "some-step",
			WorkingDirectory:     "some-working-dir",
			EnvironmentVariables: []string{"some", "env", "var"},
		}
		fakeImageFetchingDelegate = new(wfakes.FakeImageFetchingDelegate)
		fakeImageFetchingDelegate.StderrReturns(stderrBuf)
		fakeWorker = new(wfakes.FakeClient)
		customTypes = atc.ResourceTypes{
			{
				Name:   "custom-type-a",
				Type:   "base-type",
				Source: atc.Source{"some": "source"},
			},
			{
				Name:   "custom-type-b",
				Type:   "custom-type-a",
				Source: atc.Source{"some": "source"},
			},
		}
	})

	JustBeforeEach(func() {
		fetchedImage, fetchErr = fetcher.FetchImage(
			logger,
			imageConfig,
			signals,
			identifier,
			metadata,
			fakeImageFetchingDelegate,
			fakeWorker,
			customTypes,
		)
	})

	Context("when initializing the Check resource works", func() {
		var (
			fakeCheckResource *rfakes.FakeResource
		)

		BeforeEach(func() {
			fakeCheckResource = new(rfakes.FakeResource)
			fakeImageTracker.InitReturns(fakeCheckResource, nil)
		})

		Context("when check returns a version", func() {
			var (
				fakeVersionedSource *rfakes.FakeVersionedSource
			)

			BeforeEach(func() {
				fakeCheckResource.CheckReturns([]atc.Version{{"v": "1"}}, nil)
			})

			Context("when saving the version in the database succeeds", func() {
				BeforeEach(func() {
					fakeImageFetchingDelegate.ImageVersionDeterminedReturns(nil)
				})

				Context("when initializing the Get resource works", func() {
					var (
						fakeGetResource *rfakes.FakeResource
						fakeCache       *rfakes.FakeCache
					)

					BeforeEach(func() {
						fakeGetResource = new(rfakes.FakeResource)
						fakeCache = new(rfakes.FakeCache)
						fakeImageTracker.InitWithCacheReturns(fakeGetResource, fakeCache, nil)

						fakeVersionedSource = new(rfakes.FakeVersionedSource)
						fakeGetResource.GetReturns(fakeVersionedSource)
					})

					Context("when the 'get' source provides a metadata.json", func() {
						BeforeEach(func() {
							fakeVersionedSource.StreamOutReturns(tarStreamWith(`{"env": ["why=does", "this=package", "dot=import", "everything=why?"], "user":"pilot"}`), nil)
						})

						Context("when the cache is not initialized", func() {
							BeforeEach(func() {
								fakeCache.IsInitializedReturns(false, nil)
							})

							Context("when the 'get' action completes successfully", func() {
								BeforeEach(func() {
									fakeVersionedSource.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
										Expect(fakeCache.InitializeCallCount()).To(Equal(0))
										close(ready)
										return nil
									}
								})

								Context("when the cache cannot be initialized", func() {
									var cacheFail error

									BeforeEach(func() {
										cacheFail = errors.New("boom! cache.Initialize error")
										fakeCache.InitializeReturns(cacheFail)
									})

									It("returns an error when cache initialization fails", func() {
										Expect(fetchedImage).To(BeNil())
										Expect(fetchErr).To(Equal(cacheFail))
										Expect(fakeGetResource.CacheVolumeCallCount()).To(Equal(0))
									})
								})

								Context("when the resource has a volume", func() {
									var (
										fakeVolume *wfakes.FakeVolume
										volumePath string
									)

									BeforeEach(func() {
										fakeVolume = new(wfakes.FakeVolume)
										volumePath = "C:/Documents and Settings/Evan/My Documents"

										fakeVolume.PathReturns(volumePath)
										fakeGetResource.CacheVolumeReturns(fakeVolume, true)
									})

									It("succeeds", func() {
										Expect(fetchErr).To(BeNil())
									})

									It("has the correct env on the image", func() {
										Expect(fetchedImage.Metadata()).To(Equal(worker.ImageMetadata{
											Env:  []string{"why=does", "this=package", "dot=import", "everything=why?"},
											User: "pilot",
										}))
									})

									It("creates a tracker for checking and getting the image resource", func() {
										Expect(fakeTrackerFactory.TrackerForCallCount()).To(Equal(1))
										Expect(fakeTrackerFactory.TrackerForArgsForCall(0)).To(Equal(fakeWorker))
									})

									It("created the 'check' resource with the correct session, with the currently fetching type removed from the set", func() {
										Expect(fakeImageTracker.InitCallCount()).To(Equal(1))
										_, metadata, session, resourceType, tags, actualCustomTypes, delegate := fakeImageTracker.InitArgsForCall(0)
										Expect(metadata).To(Equal(resource.EmptyMetadata{}))
										Expect(session).To(Equal(resource.Session{
											ID: worker.Identifier{
												BuildID:             1234,
												PlanID:              "some-plan-id",
												ImageResourceType:   "docker",
												ImageResourceSource: atc.Source{"some": "source"},
												Stage:               db.ContainerStageCheck,
											},
											Metadata: worker.Metadata{
												PipelineName:         "some-pipeline",
												Type:                 db.ContainerTypeCheck,
												StepName:             "some-step",
												WorkingDirectory:     "",  // figure this out once we actually support hijacking these
												EnvironmentVariables: nil, // figure this out once we actually support hijacking these
											},
										}))
										Expect(resourceType).To(Equal(resource.ResourceType("docker")))
										Expect(tags).To(BeNil())
										Expect(actualCustomTypes).To(Equal(customTypes))
										Expect(delegate).To(Equal(fakeImageFetchingDelegate))
									})

									It("ran 'check' with the right config", func() {
										Expect(fakeCheckResource.CheckCallCount()).To(Equal(1))
										checkSource, checkVersion := fakeCheckResource.CheckArgsForCall(0)
										Expect(checkVersion).To(BeNil())
										Expect(checkSource).To(Equal(imageConfig.Source))
									})

									It("saved the image resource version in the database", func() {
										expectedIdentifier := db.VolumeIdentifier{
											ResourceVersion: atc.Version{"v": "1"},
											ResourceHash:    `docker{"some":"source"}`,
										}
										Expect(fakeImageFetchingDelegate.ImageVersionDeterminedCallCount()).To(Equal(1))
										Expect(fakeImageFetchingDelegate.ImageVersionDeterminedArgsForCall(0)).To(Equal(expectedIdentifier))
									})

									It("releases the check resource, which includes releasing its volume", func() {
										Expect(fakeCheckResource.ReleaseCallCount()).To(Equal(1))
									})

									It("marks the cache as initialized", func() {
										Expect(fakeCache.InitializeCallCount()).To(Equal(1))
									})

									It("created the 'get' resource with the correct session", func() {
										Expect(fakeImageTracker.InitWithCacheCallCount()).To(Equal(1))
										_, metadata, session, resourceType, tags, cacheID, actualCustomTypes, delegate := fakeImageTracker.InitWithCacheArgsForCall(0)
										Expect(metadata).To(Equal(resource.EmptyMetadata{}))
										Expect(session).To(Equal(resource.Session{
											ID: worker.Identifier{
												BuildID:             1234,
												PlanID:              "some-plan-id",
												ImageResourceType:   "docker",
												ImageResourceSource: atc.Source{"some": "source"},
												Stage:               db.ContainerStageGet,
											},
											Metadata: worker.Metadata{
												PipelineName:         "some-pipeline",
												Type:                 db.ContainerTypeGet,
												StepName:             "some-step",
												WorkingDirectory:     "",  // figure this out once we actually support hijacking these
												EnvironmentVariables: nil, // figure this out once we actually support hijacking these
											},
										}))
										Expect(resourceType).To(Equal(resource.ResourceType("docker")))
										Expect(tags).To(BeNil())
										Expect(cacheID).To(Equal(resource.ResourceCacheIdentifier{
											Type:    "docker",
											Version: atc.Version{"v": "1"},
											Source:  atc.Source{"some": "source"},
										}))
										Expect(actualCustomTypes).To(Equal(customTypes))
										Expect(delegate).To(Equal(fakeImageFetchingDelegate))
									})

									It("constructs the 'get' runner", func() {
										Expect(fakeGetResource.GetCallCount()).To(Equal(1))
										ioConfig, getSource, params, getVersion := fakeGetResource.GetArgsForCall(0)
										Expect(getVersion).To(Equal(atc.Version{"v": "1"}))
										Expect(params).To(BeNil())
										Expect(getSource).To(Equal(imageConfig.Source))
										Expect(ioConfig).To(Equal(resource.IOConfig{
											Stderr: stderrBuf,
										}))
									})

									It("ran the 'get' action, forwarding signal and ready channels", func() {
										Expect(fakeVersionedSource.RunCallCount()).To(Equal(1))
										signals, ready := fakeVersionedSource.RunArgsForCall(0)
										Expect(signals).ToNot(BeNil())
										Expect(ready).ToNot(BeNil())
									})

									It("marks the cache as initialized", func() {
										Expect(fakeCache.InitializeCallCount()).To(Equal(1))
									})

									It("gets the volume", func() {
										Expect(fakeGetResource.CacheVolumeCallCount()).To(Equal(1))
									})

									It("creates the container with the volume's path as the rootFS", func() {
										Expect(fakeGetResource.CacheVolumeCallCount()).To(Equal(1))
									})

									Describe("releasing the image", func() {
										It("releases the get resource", func() {
											finalTTL := 5 * time.Second
											fetchedImage.Release(&finalTTL)

											Expect(fakeGetResource.ReleaseCallCount()).To(Equal(1))
											Expect(fakeGetResource.ReleaseArgsForCall(0)).To(Equal(&finalTTL))
										})
									})

									Context("when the metadata.json is bogus", func() {
										BeforeEach(func() {
											fakeVersionedSource.StreamOutReturns(tarStreamWith(`{"env": 42}`), nil)
										})

										It("returns ErrMalformedMetadata", func() {
											Expect(fetchErr).To(BeAssignableToTypeOf(image.MalformedMetadataError{}))
											Expect(fetchErr.Error()).To(Equal(fmt.Sprintf("malformed image metadata: json: cannot unmarshal number into Go value of type []string")))
										})
									})

									Context("when streaming the metadata out fails", func() {
										disaster := errors.New("nope")

										BeforeEach(func() {
											fakeVersionedSource.StreamOutReturns(nil, disaster)
										})

										It("returns the error", func() {
											Expect(fetchErr).To(Equal(disaster))
										})
									})
								})

								Context("when the resource still does not have a volume for some reason", func() {
									BeforeEach(func() {
										fakeGetResource.CacheVolumeReturns(nil, false)
									})

									It("returns an appropriate error", func() {
										Expect(fetchErr).To(Equal(image.ErrImageGetDidNotProduceVolume))
									})
								})
							})

							Context("when the 'get' action fails", func() {
								var (
									disaster error
								)

								BeforeEach(func() {
									disaster = errors.New("wah")
									fakeVersionedSource.RunReturns(disaster)
								})

								It("returns the error", func() {
									Expect(fetchErr).To(Equal(disaster))
								})
							})
						})

						Context("when the cache is initialized", func() {
							BeforeEach(func() {
								fakeCache.IsInitializedReturns(true, nil)
							})

							Context("when the resource has a volume", func() { // TODO: corresponding negative case(s)?
								var (
									fakeVolume *wfakes.FakeVolume
									volumePath string
								)

								BeforeEach(func() {
									fakeVolume = new(wfakes.FakeVolume)
									volumePath = "C:/Documents and Settings/Evan/My Documents"

									fakeVolume.PathReturns(volumePath)
									fakeGetResource.CacheVolumeReturns(fakeVolume, true)
								})

								It("does not run the 'get' runner", func() {
									Expect(fakeVersionedSource.RunCallCount()).To(Equal(0))
								})

								It("does not mark the cache as initialized again", func() {
									Expect(fakeCache.InitializeCallCount()).To(Equal(0))
								})
							})
						})
					})

					Context("when checking if the cache is initialized fails", func() {
						var (
							disaster error
						)

						BeforeEach(func() {
							disaster = errors.New("wah")
							fakeCache.IsInitializedReturns(false, disaster)
						})

						It("returns the error", func() {
							Expect(fetchErr).To(Equal(disaster))
						})
					})
				})

				Context("when initializing the Get resource fails", func() {
					var (
						disaster error
					)

					BeforeEach(func() {
						disaster = errors.New("wah")
						fakeImageTracker.InitWithCacheReturns(nil, nil, disaster)
					})

					It("returns the error", func() {
						Expect(fetchErr).To(Equal(disaster))
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
					Expect(fakeImageTracker.InitWithCacheCallCount()).To(Equal(0))
				})
			})
		})

		Context("when check returns no versions", func() {
			BeforeEach(func() {
				fakeCheckResource.CheckReturns([]atc.Version{}, nil)
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
			})

			It("returns the error", func() {
				Expect(fetchErr).To(Equal(disaster))
			})

			It("does not construct the 'get' resource", func() {
				Expect(fakeImageTracker.InitWithCacheCallCount()).To(Equal(0))
			})
		})
	})

	Context("when initializing the Check resource fails", func() {
		var (
			disaster error
		)

		BeforeEach(func() {
			disaster = errors.New("wah")
			fakeImageTracker.InitReturns(nil, disaster)
		})

		It("returns the error", func() {
			Expect(fetchErr).To(Equal(disaster))
		})

		It("does not construct the 'get' resource", func() {
			Expect(fakeImageTracker.InitWithCacheCallCount()).To(Equal(0))
		})
	})
})

func tarStreamWith(metadata string) io.ReadCloser {
	buffer := gbytes.NewBuffer()

	tarWriter := tar.NewWriter(buffer)
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

	return buffer
}
