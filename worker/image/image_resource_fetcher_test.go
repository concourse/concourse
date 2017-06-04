package image_test

import (
	"archive/tar"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/resource/resourcefakes"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/image"
	"github.com/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Image", func() {
	var fakeResourceFactory *resourcefakes.FakeResourceFactory
	var fakeImageResource *resourcefakes.FakeResource
	var fakeResourceFetcherFactory *resourcefakes.FakeFetcherFactory
	var fakeResourceFetcher *resourcefakes.FakeFetcher
	var fakeResourceFactoryFactory *resourcefakes.FakeResourceFactoryFactory
	var fakeResourceCacheFactory *dbfakes.FakeResourceCacheFactory
	var fakeResourceConfigFactory *dbfakes.FakeResourceConfigFactory
	var fakeCreatingContainer *dbfakes.FakeCreatingContainer

	var imageResourceFetcher image.ImageResourceFetcher

	var stderrBuf *gbytes.Buffer

	var logger lager.Logger
	var imageResource atc.ImageResource
	var signals <-chan os.Signal
	var fakeImageFetchingDelegate *workerfakes.FakeImageFetchingDelegate
	var fakeWorker *workerfakes.FakeWorker
	var fakeClock *fakeclock.FakeClock
	var customTypes atc.VersionedResourceTypes
	var privileged bool

	var fetchedVolume worker.Volume
	var fetchedMetadataReader io.ReadCloser
	var fetchedVersion atc.Version
	var fetchErr error
	var teamID int

	BeforeEach(func() {
		fakeResourceFactory = new(resourcefakes.FakeResourceFactory)
		fakeImageResource = new(resourcefakes.FakeResource)
		fakeResourceFetcherFactory = new(resourcefakes.FakeFetcherFactory)
		fakeResourceFetcher = new(resourcefakes.FakeFetcher)
		fakeResourceFactoryFactory = new(resourcefakes.FakeResourceFactoryFactory)
		fakeResourceConfigFactory = new(dbfakes.FakeResourceConfigFactory)
		fakeResourceFetcherFactory.FetcherForReturns(fakeResourceFetcher)
		fakeResourceFactoryFactory.FactoryForReturns(fakeResourceFactory)
		fakeCreatingContainer = new(dbfakes.FakeCreatingContainer)
		fakeClock = fakeclock.NewFakeClock(time.Now())
		stderrBuf = gbytes.NewBuffer()

		logger = lagertest.NewTestLogger("test")
		imageResource = atc.ImageResource{
			Type:   "docker",
			Source: atc.Source{"some": "source"},
		}
		signals = make(chan os.Signal)
		fakeImageFetchingDelegate = new(workerfakes.FakeImageFetchingDelegate)
		fakeImageFetchingDelegate.StderrReturns(stderrBuf)
		fakeWorker = new(workerfakes.FakeWorker)
		fakeWorker.TagsReturns(atc.Tags{"worker", "tags"})
		teamID = 123

		customTypes = atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Name:   "custom-type-a",
					Type:   "base-type",
					Source: atc.Source{"some": "source"},
				},
				Version: atc.Version{"some": "version"},
			},
			{
				ResourceType: atc.ResourceType{
					Name:   "custom-type-b",
					Type:   "custom-type-a",
					Source: atc.Source{"some": "source"},
				},
				Version: atc.Version{"some": "version"},
			},
		}

		fakeResourceCacheFactory = new(dbfakes.FakeResourceCacheFactory)

		imageResourceFetcher = image.NewImageResourceFetcherFactory(
			fakeResourceFetcherFactory,
			fakeResourceFactoryFactory,
			fakeResourceCacheFactory,
			fakeResourceConfigFactory,
			fakeClock,
		).NewImageResourceFetcher(
			fakeWorker,
			db.ForBuild(42),
			imageResource,
			teamID,
			customTypes,
			fakeImageFetchingDelegate,
		)
	})

	JustBeforeEach(func() {
		fetchedVolume, fetchedMetadataReader, fetchedVersion, fetchErr = imageResourceFetcher.Fetch(
			logger,
			signals,
			fakeCreatingContainer,
			privileged,
		)
	})

	Context("when acquiring resource checking lock succeeds", func() {
		BeforeEach(func() {
			fakeLock := new(lockfakes.FakeLock)
			fakeResourceConfigFactory.AcquireResourceCheckingLockReturns(fakeLock, true, nil)
		})

		Context("when initializing the Check resource works", func() {
			var (
				fakeCheckResource *resourcefakes.FakeResource
				fakeBuildResource *resourcefakes.FakeResource
			)

			BeforeEach(func() {
				fakeCheckResource = new(resourcefakes.FakeResource)
				fakeBuildResource = new(resourcefakes.FakeResource)
				fakeResourceFactory.NewCheckResourceReturns(fakeCheckResource, nil)
			})

			Context("when check returns a version", func() {
				BeforeEach(func() {
					fakeCheckResource.CheckReturns([]atc.Version{{"v": "1"}}, nil)
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
							fakeVersionedSource *resourcefakes.FakeVersionedSource
						)

						BeforeEach(func() {
							fakeVersionedSource = new(resourcefakes.FakeVersionedSource)
							fakeResourceFetcher.FetchReturns(fakeVersionedSource, nil)

							fakeVersionedSource.StreamOutReturns(tarStreamWith("some-tar-contents"), nil)
							fakeVolume := new(workerfakes.FakeVolume)
							fakeVersionedSource.VolumeReturns(fakeVolume)
						})

						Context("when the resource has a volume", func() {
							var (
								fakeVolume *workerfakes.FakeVolume
								volumePath string
							)

							BeforeEach(func() {
								fakeVolume = new(workerfakes.FakeVolume)
								volumePath = "C:/Documents and Settings/Evan/My Documents"

								fakeVolume.PathReturns(volumePath)
								fakeVersionedSource.VolumeReturns(fakeVolume)

								privileged = true
							})

							Context("calling NewBuildResource", func() {
								BeforeEach(func() {
									fakeResourceFactory.NewCheckResourceReturns(fakeBuildResource, nil)
								})

								It("created the 'check' resource with the correct session, with the currently fetching type removed from the set", func() {
									Expect(fakeResourceFactory.NewCheckResourceCallCount()).To(Equal(1))
									_, csig, user, _, _, metadata, resourceSpec, actualCustomTypes, delegate := fakeResourceFactory.NewCheckResourceArgsForCall(0)
									Expect(csig).To(Equal(signals))
									Expect(user).To(Equal(db.ForBuild(42)))
									Expect(metadata).To(Equal(db.ContainerMetadata{
										Type: db.ContainerTypeCheck,
									}))
									Expect(resourceSpec).To(Equal(worker.ContainerSpec{
										ImageSpec: worker.ImageSpec{
											ResourceType: "docker",
										},
										Tags:   []string{"worker", "tags"},
										TeamID: 123,
									}))
									Expect(actualCustomTypes).To(Equal(customTypes))
									Expect(delegate).To(Equal(fakeImageFetchingDelegate))
								})
							})

							Context("calling NewCheckResource", func() {
								BeforeEach(func() {
									fakeResourceFactory.NewCheckResourceReturns(fakeCheckResource, nil)
								})

								It("created the 'check' resource with the correct session, with the currently fetching type removed from the set", func() {
									Expect(fakeResourceFactory.NewCheckResourceCallCount()).To(Equal(1))
									_, csig, user, _, _, metadata, resourceSpec, actualCustomTypes, delegate := fakeResourceFactory.NewCheckResourceArgsForCall(0)
									Expect(csig).To(Equal(signals))
									Expect(user).To(Equal(db.ForBuild(42)))
									Expect(metadata).To(Equal(db.ContainerMetadata{
										Type: db.ContainerTypeCheck,
									}))
									Expect(resourceSpec).To(Equal(worker.ContainerSpec{
										ImageSpec: worker.ImageSpec{
											ResourceType: "docker",
										},
										Tags:   []string{"worker", "tags"},
										TeamID: 123,
									}))
									Expect(actualCustomTypes).To(Equal(customTypes))
									Expect(delegate).To(Equal(fakeImageFetchingDelegate))
								})
							})

							It("succeeds", func() {
								Expect(fetchErr).To(BeNil())
							})

							It("returns the image volume", func() {
								Expect(fetchedVolume).To(Equal(fakeVolume))
							})

							It("calls StreamOut on the versioned source with the right metadata path", func() {
								Expect(fakeVersionedSource.StreamOutCallCount()).To(Equal(1))
								Expect(fakeVersionedSource.StreamOutArgsForCall(0)).To(Equal("metadata.json"))
							})

							It("returns a tar stream containing the contents of metadata.json", func() {
								Expect(ioutil.ReadAll(fetchedMetadataReader)).To(Equal([]byte("some-tar-contents")))
							})

							It("has the version on the image", func() {
								Expect(fetchedVersion).To(Equal(atc.Version{"v": "1"}))
							})

							It("created the 'check' resource with the correct session, with the currently fetching type removed from the set", func() {
								Expect(fakeResourceFactory.NewCheckResourceCallCount()).To(Equal(1))
								_, csig, user, _, _, metadata, resourceSpec, actualCustomTypes, delegate := fakeResourceFactory.NewCheckResourceArgsForCall(0)
								Expect(csig).To(Equal(signals))
								Expect(user).To(Equal(db.ForBuild(42)))
								Expect(metadata).To(Equal(db.ContainerMetadata{
									Type: db.ContainerTypeCheck,
								}))
								Expect(resourceSpec).To(Equal(worker.ContainerSpec{
									ImageSpec: worker.ImageSpec{
										ResourceType: "docker",
									},
									Tags:   []string{"worker", "tags"},
									TeamID: 123,
								}))
								Expect(actualCustomTypes).To(Equal(customTypes))
								Expect(delegate).To(Equal(fakeImageFetchingDelegate))
							})

							It("ran 'check' with the right config", func() {
								Expect(fakeCheckResource.CheckCallCount()).To(Equal(1))
								checkSource, checkVersion := fakeCheckResource.CheckArgsForCall(0)
								Expect(checkVersion).To(BeNil())
								Expect(checkSource).To(Equal(imageResource.Source))
							})

							It("saved the image resource version in the database", func() {
								expectedIdentifier := worker.ResourceCacheIdentifier{
									ResourceVersion: atc.Version{"v": "1"},
									ResourceHash:    `docker{"some":"source"}`,
								}
								Expect(fakeImageFetchingDelegate.ImageVersionDeterminedCallCount()).To(Equal(1))
								Expect(fakeImageFetchingDelegate.ImageVersionDeterminedArgsForCall(0)).To(Equal(expectedIdentifier))
							})

							It("fetches resource with correct session", func() {
								Expect(fakeResourceFetcher.FetchCallCount()).To(Equal(1))
								_, session, tags, actualTeamID, actualCustomTypes, resourceInstance, metadata, delegate, resourceOptions, _, _ := fakeResourceFetcher.FetchArgsForCall(0)
								Expect(metadata).To(Equal(resource.EmptyMetadata{}))
								Expect(session).To(Equal(resource.Session{
									Metadata: db.ContainerMetadata{
										Type: db.ContainerTypeGet,
									},
								}))
								Expect(tags).To(Equal(atc.Tags{"worker", "tags"}))
								Expect(actualTeamID).To(Equal(teamID))
								Expect(resourceInstance).To(Equal(resource.NewResourceInstance(
									"docker",
									atc.Version{"v": "1"},
									atc.Source{"some": "source"},
									atc.Params{},
									db.ForBuild(42),
									customTypes,
									fakeResourceCacheFactory,
								)))
								Expect(actualCustomTypes).To(Equal(customTypes))
								Expect(delegate).To(Equal(fakeImageFetchingDelegate))
								Expect(resourceOptions.ResourceType()).To(Equal(resource.ResourceType("docker")))
								expectedLockName := fmt.Sprintf("%x",
									sha256.Sum256([]byte(
										`{"type":"docker","version":{"v":"1"},"source":{"some":"source"},"worker_name":"fake-worker-name"}`,
									)),
								)
								Expect(resourceOptions.LockName("fake-worker-name")).To(Equal(expectedLockName))
							})

							It("gets the volume", func() {
								Expect(fakeVersionedSource.VolumeCallCount()).To(Equal(1))
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

							Context("when the resource still does not have a volume for some reason", func() {
								BeforeEach(func() {
									fakeVersionedSource.VolumeReturns(nil)
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
					Expect(fakeResourceFetcher.FetchCallCount()).To(Equal(0))
				})
			})
		})

		Context("when initializing the Check resource fails", func() {
			var (
				disaster error
			)

			BeforeEach(func() {
				disaster = errors.New("wah")
				fakeResourceFactory.NewCheckResourceReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(fetchErr).To(Equal(disaster))
			})

			It("does not construct the 'get' resource", func() {
				Expect(fakeResourceFetcher.FetchCallCount()).To(Equal(0))
			})
		})
	})

	Context("when could not acquire the lock", func() {
		var fakeLock *lockfakes.FakeLock

		BeforeEach(func() {
			fakeCheckResource := new(resourcefakes.FakeResource)
			fakeResourceFactory.NewCheckResourceReturns(fakeCheckResource, nil)

			fakeLock = new(lockfakes.FakeLock)
			callCount := 0
			fakeResourceConfigFactory.AcquireResourceCheckingLockStub = func(lager.Logger, db.ResourceUser, string, atc.Source, atc.VersionedResourceTypes) (lock.Lock, bool, error) {
				callCount++

				if callCount == 5 {
					return fakeLock, true, nil
				}

				go fakeClock.WaitForWatcherAndIncrement(time.Second)

				return nil, false, nil
			}
		})

		It("retries until it acquires the lock with delay interval", func() {
			Expect(fakeResourceConfigFactory.AcquireResourceCheckingLockCallCount()).To(Equal(5))
		})

		It("releases the lock", func() {
			Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
		})
	})

	Context("when acquiring resource checking lock fails", func() {
		var disaster = errors.New("disaster")

		BeforeEach(func() {
			fakeResourceConfigFactory.AcquireResourceCheckingLockReturns(nil, false, disaster)
		})

		It("returns the error", func() {
			Expect(fetchErr).To(Equal(disaster))
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
