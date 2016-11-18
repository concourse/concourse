package worker_test

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/garden"
	gfakes "code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/dbng/dbngfakes"
	. "github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/workerfakes"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/baggageclaimfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Worker", func() {
	var (
		logger                     *lagertest.TestLogger
		fakeGardenClient           *gfakes.FakeClient
		fakeBaggageclaimClient     *baggageclaimfakes.FakeClient
		fakeVolumeClient           *wfakes.FakeVolumeClient
		fakeImageFactory           *wfakes.FakeImageFactory
		fakeImage                  *wfakes.FakeImage
		fakeGardenWorkerDB         *wfakes.FakeGardenWorkerDB
		fakeWorkerProvider         *wfakes.FakeWorkerProvider
		fakeClock                  *fakeclock.FakeClock
		fakePipelineDBFactory      *dbfakes.FakePipelineDBFactory
		fakeDBContainerFactory     *wfakes.FakeDBContainerFactory
		fakeDBResourceCacheFactory *dbngfakes.FakeResourceCacheFactory
		fakeResourceConfigFactory  *dbngfakes.FakeResourceConfigFactory
		fakeDBVolumeFactory        *dbngfakes.FakeVolumeFactory
		activeContainers           int
		resourceTypes              []atc.WorkerResourceType
		platform                   string
		tags                       atc.Tags
		teamID                     int
		workerName                 string
		workerStartTime            int64
		httpProxyURL               string
		httpsProxyURL              string
		noProxy                    string
		origUptime                 time.Duration
		workerUptime               uint64

		gardenWorker Worker
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakeGardenClient = new(gfakes.FakeClient)
		fakeBaggageclaimClient = new(baggageclaimfakes.FakeClient)
		fakeVolumeClient = new(wfakes.FakeVolumeClient)
		fakeImageFactory = new(wfakes.FakeImageFactory)
		fakeImage = new(wfakes.FakeImage)
		fakeImageFactory.NewImageReturns(fakeImage)
		fakeGardenWorkerDB = new(wfakes.FakeGardenWorkerDB)
		fakeWorkerProvider = new(wfakes.FakeWorkerProvider)
		fakePipelineDBFactory = new(dbfakes.FakePipelineDBFactory)
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

		fakeDBContainerFactory = new(wfakes.FakeDBContainerFactory)
		fakeDBResourceCacheFactory = new(dbngfakes.FakeResourceCacheFactory)
		fakeResourceConfigFactory = new(dbngfakes.FakeResourceConfigFactory)
		fakeDBVolumeFactory = new(dbngfakes.FakeVolumeFactory)
	})

	JustBeforeEach(func() {
		containerProviderFactory := NewContainerProviderFactory(
			fakeGardenClient,
			fakeBaggageclaimClient,
			fakeVolumeClient,
			fakeImageFactory,
			fakeDBContainerFactory,
			fakeDBVolumeFactory,
			fakeGardenWorkerDB,
			fakeClock,
		)
		gardenWorker = NewGardenWorker(
			containerProviderFactory,
			fakeVolumeClient,
			fakePipelineDBFactory,
			fakeDBContainerFactory,
			fakeDBResourceCacheFactory,
			fakeResourceConfigFactory,
			fakeGardenWorkerDB,
			fakeWorkerProvider,
			fakeClock,
			activeContainers,
			resourceTypes,
			platform,
			tags,
			teamID,
			workerName,
			"1.2.3.4",
			workerStartTime,
			httpProxyURL,
			httpsProxyURL,
			noProxy,
		)

		origUptime = gardenWorker.Uptime()
		fakeClock.IncrementBySeconds(workerUptime)
	})

	Describe("LookupContainer", func() {
		var handle string

		BeforeEach(func() {
			handle = "we98lsv"
		})

		Context("when the gardenClient returns a container and no error", func() {
			var (
				fakeContainer  *gfakes.FakeContainer
				foundContainer Container
				findErr        error
				found          bool
			)

			BeforeEach(func() {
				fakeContainer = new(gfakes.FakeContainer)
				fakeContainer.HandleReturns("some-handle")

				fakeDBVolumeFactory.FindVolumesForContainerReturns([]dbng.CreatedVolume{}, nil)

				fakeDBContainerFactory.FindContainerReturns(&dbng.CreatedContainer{}, true, nil)
				fakeGardenClient.LookupReturns(fakeContainer, nil)
			})

			JustBeforeEach(func() {
				foundContainer, found, findErr = gardenWorker.LookupContainer(logger, handle)
			})

			It("returns the container and no error", func() {
				Expect(findErr).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(foundContainer.Handle()).To(Equal(fakeContainer.Handle()))
			})

			Context("when the concourse:volumes property is present", func() {
				var (
					handle1Volume         *baggageclaimfakes.FakeVolume
					handle2Volume         *baggageclaimfakes.FakeVolume
					expectedHandle1Volume Volume
					expectedHandle2Volume Volume
				)

				BeforeEach(func() {
					handle1Volume = new(baggageclaimfakes.FakeVolume)
					handle2Volume = new(baggageclaimfakes.FakeVolume)

					fakeVolume1 := new(dbngfakes.FakeCreatedVolume)
					fakeVolume2 := new(dbngfakes.FakeCreatedVolume)

					expectedHandle1Volume = NewVolume(handle1Volume, fakeVolume1)
					expectedHandle2Volume = NewVolume(handle2Volume, fakeVolume2)

					fakeVolume1.HandleReturns("handle-1")
					fakeVolume2.HandleReturns("handle-2")

					fakeVolume1.PathReturns("/handle-1/path")
					fakeVolume2.PathReturns("/handle-2/path")

					fakeDBVolumeFactory.FindVolumesForContainerReturns([]dbng.CreatedVolume{fakeVolume1, fakeVolume2}, nil)

					fakeBaggageclaimClient.LookupVolumeStub = func(logger lager.Logger, handle string) (baggageclaim.Volume, bool, error) {
						if handle == "handle-1" {
							return handle1Volume, true, nil
						} else if handle == "handle-2" {
							return handle2Volume, true, nil
						} else {
							panic("unknown handle: " + handle)
						}
					}
				})

				Describe("VolumeMounts", func() {
					It("returns all bound volumes based on properties on the container", func() {
						Expect(foundContainer.VolumeMounts()).To(ConsistOf([]VolumeMount{
							{Volume: expectedHandle1Volume, MountPath: "/handle-1/path"},
							{Volume: expectedHandle2Volume, MountPath: "/handle-2/path"},
						}))
					})

					Context("when LookupVolume returns an error", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeBaggageclaimClient.LookupVolumeReturns(nil, false, disaster)
						})

						It("returns the error on lookup", func() {
							Expect(findErr).To(Equal(disaster))
						})
					})
				})
			})

			Context("when the user property is present", func() {
				var (
					actualSpec garden.ProcessSpec
					actualIO   garden.ProcessIO
				)

				BeforeEach(func() {
					actualSpec = garden.ProcessSpec{
						Path: "some-path",
						Args: []string{"some", "args"},
						Env:  []string{"some=env"},
						Dir:  "some-dir",
					}

					actualIO = garden.ProcessIO{}

					fakeContainer.PropertiesReturns(garden.Properties{"user": "maverick"}, nil)
				})

				JustBeforeEach(func() {
					foundContainer.Run(actualSpec, actualIO)
				})

				Describe("Run", func() {
					It("calls Run() on the garden container and injects the user", func() {
						Expect(fakeContainer.RunCallCount()).To(Equal(1))
						spec, io := fakeContainer.RunArgsForCall(0)
						Expect(spec).To(Equal(garden.ProcessSpec{
							Path: "some-path",
							Args: []string{"some", "args"},
							Env:  []string{"some=env"},
							Dir:  "some-dir",
							User: "maverick",
						}))
						Expect(io).To(Equal(garden.ProcessIO{}))
					})
				})
			})

			Context("when the user property is not present", func() {
				var (
					actualSpec garden.ProcessSpec
					actualIO   garden.ProcessIO
				)

				BeforeEach(func() {
					actualSpec = garden.ProcessSpec{
						Path: "some-path",
						Args: []string{"some", "args"},
						Env:  []string{"some=env"},
						Dir:  "some-dir",
					}

					actualIO = garden.ProcessIO{}

					fakeContainer.PropertiesReturns(garden.Properties{"user": ""}, nil)
				})

				JustBeforeEach(func() {
					foundContainer.Run(actualSpec, actualIO)
				})

				Describe("Run", func() {
					It("calls Run() on the garden container and injects the default user", func() {
						Expect(fakeContainer.RunCallCount()).To(Equal(1))
						spec, io := fakeContainer.RunArgsForCall(0)
						Expect(spec).To(Equal(garden.ProcessSpec{
							Path: "some-path",
							Args: []string{"some", "args"},
							Env:  []string{"some=env"},
							Dir:  "some-dir",
							User: "root",
						}))
						Expect(io).To(Equal(garden.ProcessIO{}))
						Expect(fakeContainer.RunCallCount()).To(Equal(1))
					})
				})
			})
		})

		Context("when the gardenClient returns garden.ContainerNotFoundError", func() {
			BeforeEach(func() {
				fakeGardenClient.LookupReturns(nil, garden.ContainerNotFoundError{Handle: "some-handle"})
			})

			It("returns false and no error", func() {
				_, found, err := gardenWorker.LookupContainer(logger, handle)
				Expect(err).ToNot(HaveOccurred())

				Expect(found).To(BeFalse())
			})
		})

		Context("when the gardenClient returns an error", func() {
			var expectedErr error

			BeforeEach(func() {
				expectedErr = fmt.Errorf("container not found")
				fakeGardenClient.LookupReturns(nil, expectedErr)
			})

			It("returns nil and forwards the error", func() {
				foundContainer, _, err := gardenWorker.LookupContainer(logger, handle)
				Expect(err).To(Equal(expectedErr))

				Expect(foundContainer).To(BeNil())
			})
		})
	})

	Describe("ValidateResourceCheckVersion", func() {
		var (
			container db.SavedContainer
			valid     bool
			checkErr  error
		)

		BeforeEach(func() {
			container = db.SavedContainer{
				Container: db.Container{
					ContainerIdentifier: db.ContainerIdentifier{
						ResourceTypeVersion: atc.Version{
							"custom-type": "some-version",
						},
						CheckType: "custom-type",
					},
					ContainerMetadata: db.ContainerMetadata{
						WorkerName: "some-worker",
					},
				},
			}
		})

		JustBeforeEach(func() {
			valid, checkErr = gardenWorker.ValidateResourceCheckVersion(container)
		})

		Context("when not a check container", func() {
			BeforeEach(func() {
				container = db.SavedContainer{
					Container: db.Container{
						ContainerMetadata: db.ContainerMetadata{
							WorkerName: "some-worker",
							Type:       db.ContainerTypeTask,
						},
					},
				}
			})

			It("returns true", func() {
				Expect(valid).To(BeTrue())
				Expect(checkErr).NotTo(HaveOccurred())
			})
		})

		Context("when container version matches worker's", func() {
			BeforeEach(func() {
				container = db.SavedContainer{
					Container: db.Container{
						ContainerIdentifier: db.ContainerIdentifier{
							ResourceTypeVersion: atc.Version{
								"some-resource": "some-version",
							},
							CheckType: "some-resource",
						},
						ContainerMetadata: db.ContainerMetadata{
							WorkerName: "some-worker",
							Type:       db.ContainerTypeCheck,
						},
					},
				}
			})

			It("returns true", func() {
				Expect(valid).To(BeTrue())
				Expect(checkErr).NotTo(HaveOccurred())
			})
		})

		Context("when container version does not match worker's", func() {
			BeforeEach(func() {
				container = db.SavedContainer{
					Container: db.Container{
						ContainerIdentifier: db.ContainerIdentifier{
							ResourceTypeVersion: atc.Version{
								"some-resource": "some-other-version",
							},
							CheckType: "some-resource",
						},
						ContainerMetadata: db.ContainerMetadata{
							WorkerName: "some-worker",
							Type:       db.ContainerTypeCheck,
						},
					},
				}
			})

			It("returns false", func() {
				Expect(valid).To(BeFalse())
				Expect(checkErr).NotTo(HaveOccurred())
			})
		})

		Context("when worker does not provide version for the resource type", func() {
			BeforeEach(func() {
				container = db.SavedContainer{
					Container: db.Container{
						ContainerIdentifier: db.ContainerIdentifier{
							ResourceTypeVersion: atc.Version{
								"some-other-resource": "some-other-version",
							},
							CheckType: "some-other-resource",
						},
						ContainerMetadata: db.ContainerMetadata{
							WorkerName: "some-worker",
							Type:       db.ContainerTypeCheck,
						},
					},
				}
			})

			It("returns false", func() {
				Expect(valid).To(BeFalse())
				Expect(checkErr).NotTo(HaveOccurred())
			})
		})

		Context("when container belongs to pipeline", func() {
			BeforeEach(func() {
				container = db.SavedContainer{
					Container: db.Container{
						ContainerIdentifier: db.ContainerIdentifier{
							ResourceTypeVersion: atc.Version{
								"some-resource": "some-version",
							},
							CheckType: "some-resource",
						},
						ContainerMetadata: db.ContainerMetadata{
							WorkerName: "some-worker",
							Type:       db.ContainerTypeCheck,
							PipelineID: 1,
						},
					},
				}
			})

			Context("when failing to get pipeline from database", func() {
				BeforeEach(func() {
					fakeGardenWorkerDB.GetPipelineByIDReturns(db.SavedPipeline{}, errors.New("disaster"))
				})

				It("returns an error", func() {
					Expect(checkErr).To(HaveOccurred())
					Expect(checkErr.Error()).To(ContainSubstring("disaster"))
				})

			})

			Context("when pipeline was found", func() {
				var fakePipelineDB *dbfakes.FakePipelineDB
				BeforeEach(func() {
					fakePipelineDB = new(dbfakes.FakePipelineDB)
					fakePipelineDBFactory.BuildReturns(fakePipelineDB)
				})

				Context("resource type is not found", func() {
					BeforeEach(func() {
						fakePipelineDB.GetResourceTypeReturns(db.SavedResourceType{}, false, nil)
					})

					Context("when worker version matches", func() {
						BeforeEach(func() {
							container.Container.ResourceTypeVersion["some-resource"] = "some-version"
						})

						It("returns true", func() {
							Expect(valid).To(BeTrue())
							Expect(checkErr).NotTo(HaveOccurred())
						})
					})

					Context("when worker version does not match", func() {
						BeforeEach(func() {
							container.Container.ResourceTypeVersion["some-resource"] = "some-other-version"
						})

						It("returns false", func() {
							Expect(valid).To(BeFalse())
							Expect(checkErr).NotTo(HaveOccurred())
						})
					})
				})

				Context("resource type is found", func() {
					BeforeEach(func() {
						fakePipelineDB.GetResourceTypeReturns(db.SavedResourceType{}, true, nil)
					})

					It("returns true", func() {
						Expect(valid).To(BeTrue())
						Expect(checkErr).NotTo(HaveOccurred())
					})
				})

				Context("getting resource type fails", func() {
					BeforeEach(func() {
						fakePipelineDB.GetResourceTypeReturns(db.SavedResourceType{}, false, errors.New("disaster"))
					})

					It("returns false and error", func() {
						Expect(valid).To(BeFalse())
						Expect(checkErr).To(HaveOccurred())
						Expect(checkErr.Error()).To(ContainSubstring("disaster"))
					})
				})
			})
		})
	})

	Describe("CreateBuildContainer", func() {
		var container Container
		var createErr error
		var imageSpec ImageSpec

		JustBeforeEach(func() {
			container, createErr = gardenWorker.CreateBuildContainer(
				logger,
				nil,
				nil,
				Identifier{},
				Metadata{},
				ContainerSpec{
					ImageSpec: imageSpec,
				},
				atc.ResourceTypes{},
				map[string]string{},
			)
		})

		BeforeEach(func() {
			fakeContainer := new(gfakes.FakeContainer)
			fakeGardenClient.CreateReturns(fakeContainer, nil)
			fakeCreatedContainer := &dbng.CreatedContainer{ID: 42}
			fakeDBContainerFactory.ContainerCreatedReturns(fakeCreatedContainer, nil)
		})

		Describe("fetching image", func() {
			Context("when image artifact source is specified in imageSpec", func() {
				var imageArtifactSource *wfakes.FakeArtifactSource
				var imageVolume *wfakes.FakeVolume
				var metadataReader io.ReadCloser

				BeforeEach(func() {

					imageArtifactSource = new(wfakes.FakeArtifactSource)

					metadataReader = ioutil.NopCloser(strings.NewReader(`{"env":["some","env"]}`))
					imageArtifactSource.StreamFileReturns(metadataReader, nil)

					imageVolume = new(wfakes.FakeVolume)
					imageVolume.PathReturns("/var/vcap/some-path")
					imageVolume.HandleReturns("some-handle")

					imageSpec = ImageSpec{
						ImageArtifactSource: imageArtifactSource,
						ImageArtifactName:   "some-image-artifact-name",
					}
				})

				Context("when the image artifact is not found in a volume on the worker", func() {
					BeforeEach(func() {
						imageArtifactSource.VolumeOnReturns(nil, false, nil)
						fakeVolumeClient.FindOrCreateVolumeForContainerReturns(imageVolume, nil)
					})

					It("looks for an existing image volume on the worker", func() {
						Expect(imageArtifactSource.VolumeOnCallCount()).To(Equal(1))
					})

					It("checks whether the artifact is in a volume on the worker", func() {
						Expect(imageArtifactSource.VolumeOnCallCount()).To(Equal(1))
						Expect(imageArtifactSource.VolumeOnArgsForCall(0)).To(Equal(gardenWorker))
					})

					Context("when streaming the artifact source to the volume fails", func() {
						var disaster error
						BeforeEach(func() {
							disaster = errors.New("this is bad")
							imageArtifactSource.StreamToReturns(disaster)
						})

						It("returns the error", func() {
							Expect(createErr).To(Equal(disaster))
						})
					})

					Context("when streaming the artifact source to the volume succeeds", func() {
						BeforeEach(func() {
							imageArtifactSource.StreamToReturns(nil)
						})

						Context("when streaming the metadata from the worker fails", func() {
							var disaster error
							BeforeEach(func() {
								disaster = errors.New("got em")
								imageArtifactSource.StreamFileReturns(nil, disaster)
							})

							It("returns the error", func() {
								Expect(createErr).To(Equal(disaster))
							})
						})

						Context("when streaming the metadata from the worker succeeds", func() {
							It("creates container with image volume and metadata", func() {
								Expect(createErr).ToNot(HaveOccurred())

								Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
								gardenSpec := fakeGardenClient.CreateArgsForCall(0)
								Expect(gardenSpec.Env).To(Equal([]string{"some", "env"}))
								Expect(gardenSpec.RootFSPath).To(Equal("raw:///var/vcap/some-path/rootfs"))
							})
						})
					})
				})

				Context("when the image artifact is in a volume on the worker", func() {
					var imageVolume *wfakes.FakeVolume
					BeforeEach(func() {
						metadataReader = ioutil.NopCloser(strings.NewReader(`{"env":["some","env"]}`))
						imageArtifactSource.StreamFileReturns(metadataReader, nil)

						artifactVolume := new(wfakes.FakeVolume)
						imageArtifactSource.VolumeOnReturns(artifactVolume, true, nil)

						imageVolume = new(wfakes.FakeVolume)
						imageVolume.PathReturns("/var/vcap/some-path")
						imageVolume.HandleReturns("some-handle")
						fakeVolumeClient.FindOrCreateVolumeForContainerReturns(imageVolume, nil)
					})

					It("looks for an existing image volume on the worker", func() {
						Expect(imageArtifactSource.VolumeOnCallCount()).To(Equal(1))
					})

					It("checks whether the artifact is in a volume on the worker", func() {
						Expect(imageArtifactSource.VolumeOnCallCount()).To(Equal(1))
						Expect(imageArtifactSource.VolumeOnArgsForCall(0)).To(Equal(gardenWorker))
					})

					Context("when streaming the metadata from the worker fails", func() {
						var disaster error
						BeforeEach(func() {
							disaster = errors.New("got em")
							imageArtifactSource.StreamFileReturns(nil, disaster)
						})

						It("returns the error", func() {
							Expect(createErr).To(Equal(disaster))
						})
					})

					Context("when streaming the metadata from the worker succeeds", func() {
						BeforeEach(func() {
							imageArtifactSource.StreamFileReturns(metadataReader, nil)
						})

						It("creates container with image volume and metadata", func() {
							Expect(createErr).ToNot(HaveOccurred())

							Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
							gardenSpec := fakeGardenClient.CreateArgsForCall(0)
							Expect(gardenSpec.Env).To(Equal([]string{"some", "env"}))
							Expect(gardenSpec.RootFSPath).To(Equal("raw:///var/vcap/some-path/rootfs"))
						})
					})
				})
			})

			Context("when image resource is specified in imageSpec", func() {
				var imageResource *atc.ImageResource

				BeforeEach(func() {
					imageResource = &atc.ImageResource{
						Type: "some-resource",
					}

					imageSpec = ImageSpec{
						ImageResource: imageResource,
					}
				})

				It("creates an image from the image resource", func() {
					Expect(fakeImageFactory.NewImageCallCount()).To(Equal(1))
					Expect(fakeImageFactory.NewImageCallCount()).To(Equal(1))
					_, _, imageResourceArg, _, _, _, _, _, _, _, _ := fakeImageFactory.NewImageArgsForCall(0)
					Expect(imageResourceArg).To(Equal(*imageResource))
				})
			})

			Context("when worker resource type is specified in image spec", func() {
				var importVolume *wfakes.FakeVolume

				BeforeEach(func() {
					imageSpec = ImageSpec{
						ResourceType: "some-resource",
					}
					importVolume = new(wfakes.FakeVolume)
					fakeVolumeClient.FindOrCreateVolumeForBaseResourceTypeReturns(importVolume, nil)
					cowVolume := new(wfakes.FakeVolume)
					cowVolume.PathReturns("/var/vcap/some-path/rootfs")
					fakeVolumeClient.FindOrCreateVolumeForContainerReturns(cowVolume, nil)
				})

				It("creates container with base resource type volume", func() {
					Expect(createErr).ToNot(HaveOccurred())
					Expect(fakeVolumeClient.FindOrCreateVolumeForBaseResourceTypeCallCount()).To(Equal(1))

					Expect(fakeVolumeClient.FindOrCreateVolumeForContainerCallCount()).To(Equal(1))
					_, volumeSpec, _, _, _ := fakeVolumeClient.FindOrCreateVolumeForContainerArgsForCall(0)
					containerRootFSStrategy, ok := volumeSpec.Strategy.(ContainerRootFSStrategy)
					Expect(ok).To(BeTrue())
					Expect(containerRootFSStrategy.Parent).To(Equal(importVolume))

					Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
					gardenSpec := fakeGardenClient.CreateArgsForCall(0)
					Expect(gardenSpec.RootFSPath).To(Equal("raw:///var/vcap/some-path/rootfs"))
				})
			})
		})
	})

	Describe("FindContainerForIdentifier", func() {
		var (
			id Identifier

			foundContainer Container
			found          bool
			lookupErr      error
		)

		BeforeEach(func() {
			id = Identifier{
				ResourceID: 1234,
			}
		})

		JustBeforeEach(func() {
			foundContainer, found, lookupErr = gardenWorker.FindContainerForIdentifier(logger, id)
		})

		Context("when the container can be found", func() {
			var (
				fakeContainer      *gfakes.FakeContainer
				fakeSavedContainer db.SavedContainer
			)

			BeforeEach(func() {
				fakeContainer = new(gfakes.FakeContainer)
				fakeContainer.HandleReturns("provider-handle")

				fakeSavedContainer = db.SavedContainer{
					Container: db.Container{
						ContainerIdentifier: db.ContainerIdentifier{
							CheckType:           "some-resource",
							ResourceTypeVersion: atc.Version{"some-resource": "some-version"},
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:     "provider-handle",
							WorkerName: "some-worker",
						},
					},
				}
				fakeWorkerProvider.FindContainerForIdentifierReturns(fakeSavedContainer, true, nil)
				fakeGardenClient.LookupReturns(fakeContainer, nil)
				fakeDBContainerFactory.FindContainerReturns(&dbng.CreatedContainer{}, true, nil)
				fakeGardenWorkerDB.GetContainerReturns(fakeSavedContainer, true, nil)
			})

			It("succeeds", func() {
				Expect(lookupErr).NotTo(HaveOccurred())
			})

			It("looks for containers with matching properties via the Garden client", func() {
				Expect(fakeWorkerProvider.FindContainerForIdentifierCallCount()).To(Equal(1))
				Expect(fakeWorkerProvider.FindContainerForIdentifierArgsForCall(0)).To(Equal(id))

				Expect(fakeGardenClient.LookupCallCount()).To(Equal(1))
				lookupHandle := fakeGardenClient.LookupArgsForCall(0)
				Expect(lookupHandle).To(Equal("provider-handle"))
			})

			Context("when container is check container", func() {
				BeforeEach(func() {
					fakeSavedContainer.Type = db.ContainerTypeCheck
					fakeWorkerProvider.FindContainerForIdentifierReturns(fakeSavedContainer, true, nil)
				})

				Context("when container resource version matches worker resource version", func() {
					It("returns container", func() {
						Expect(found).To(BeTrue())
						Expect(foundContainer.Handle()).To(Equal("provider-handle"))
					})
				})

				Context("when container resource version does not match worker resource version", func() {
					BeforeEach(func() {
						fakeSavedContainer.ResourceTypeVersion = atc.Version{"some-resource": "some-other-version"}
						fakeWorkerProvider.FindContainerForIdentifierReturns(fakeSavedContainer, true, nil)
					})

					It("does not return container", func() {
						Expect(found).To(BeFalse())
						Expect(lookupErr).NotTo(HaveOccurred())
					})
				})
			})

			Describe("the found container", func() {
				It("can be destroyed", func() {
					err := foundContainer.Destroy()
					Expect(err).NotTo(HaveOccurred())

					By("destroying via garden")
					Expect(fakeGardenClient.DestroyCallCount()).To(Equal(1))
					Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("provider-handle"))

					By("no longer heartbeating")
					fakeClock.Increment(30 * time.Second)
					Consistently(fakeContainer.SetGraceTimeCallCount).Should(Equal(1))
				})

				It("performs an initial heartbeat synchronously", func() {
					Expect(fakeContainer.SetGraceTimeCallCount()).To(Equal(1))
					Expect(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount()).To(Equal(1))
				})

				Describe("every 30 seconds", func() {
					It("heartbeats to the database and the container", func() {
						fakeClock.Increment(30 * time.Second)

						Eventually(fakeContainer.SetGraceTimeCallCount).Should(Equal(2))
						Expect(fakeContainer.SetGraceTimeArgsForCall(1)).To(Equal(5 * time.Minute))

						Eventually(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount).Should(Equal(2))
						handle, interval := fakeGardenWorkerDB.UpdateExpiresAtOnContainerArgsForCall(1)
						Expect(handle).To(Equal("provider-handle"))
						Expect(interval).To(Equal(5 * time.Minute))

						fakeClock.Increment(30 * time.Second)

						Eventually(fakeContainer.SetGraceTimeCallCount).Should(Equal(3))
						Expect(fakeContainer.SetGraceTimeArgsForCall(2)).To(Equal(5 * time.Minute))

						Eventually(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount).Should(Equal(3))
						handle, interval = fakeGardenWorkerDB.UpdateExpiresAtOnContainerArgsForCall(2)
						Expect(handle).To(Equal("provider-handle"))
						Expect(interval).To(Equal(5 * time.Minute))
					})
				})

				Describe("releasing", func() {
					It("sets a final ttl on the container and stops heartbeating", func() {
						foundContainer.Release(FinalTTL(30 * time.Minute))

						Expect(fakeContainer.SetGraceTimeCallCount()).Should(Equal(2))
						Expect(fakeContainer.SetGraceTimeArgsForCall(1)).To(Equal(30 * time.Minute))

						Expect(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount()).Should(Equal(2))
						handle, interval := fakeGardenWorkerDB.UpdateExpiresAtOnContainerArgsForCall(1)
						Expect(handle).To(Equal("provider-handle"))
						Expect(interval).To(Equal(30 * time.Minute))

						fakeClock.Increment(30 * time.Second)

						Consistently(fakeContainer.SetGraceTimeCallCount).Should(Equal(2))
						Consistently(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount).Should(Equal(2))
					})

					Context("with no final ttl", func() {
						It("does not perform a final heartbeat", func() {
							foundContainer.Release(nil)

							Consistently(fakeContainer.SetGraceTimeCallCount).Should(Equal(1))
							Consistently(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount).Should(Equal(1))
						})
					})
				})

				It("can be released multiple times", func() {
					foundContainer.Release(nil)

					Expect(func() {
						foundContainer.Release(nil)
					}).NotTo(Panic())
				})
			})
		})

		Context("when the container cannot be found", func() {
			BeforeEach(func() {
				containerToReturn := db.SavedContainer{
					Container: db.Container{
						ContainerMetadata: db.ContainerMetadata{
							Handle: "handle",
						},
					},
				}

				fakeWorkerProvider.FindContainerForIdentifierReturns(containerToReturn, true, nil)
				fakeGardenClient.LookupReturns(nil, garden.ContainerNotFoundError{Handle: "handle"})
			})

			It("expires the container and returns false and no error", func() {
				Expect(lookupErr).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(foundContainer).To(BeNil())

				expiredHandle := fakeWorkerProvider.ReapContainerArgsForCall(0)
				Expect(expiredHandle).To(Equal("handle"))
			})
		})

		Context("when looking up the container fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				containerToReturn := db.SavedContainer{
					Container: db.Container{
						ContainerMetadata: db.ContainerMetadata{
							Handle: "handle",
						},
					},
				}

				fakeWorkerProvider.FindContainerForIdentifierReturns(containerToReturn, true, nil)
				fakeGardenClient.LookupReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(lookupErr).To(Equal(disaster))
			})
		})
	})

	Describe("Satisfying", func() {
		var (
			spec WorkerSpec

			satisfyingWorker Worker
			satisfyingErr    error

			customTypes atc.ResourceTypes
		)

		BeforeEach(func() {
			spec = WorkerSpec{
				Tags:   []string{"some", "tags"},
				TeamID: teamID,
			}

			customTypes = atc.ResourceTypes{
				{
					Name:   "custom-type-b",
					Type:   "custom-type-a",
					Source: atc.Source{"some": "source"},
				},
				{
					Name:   "custom-type-a",
					Type:   "some-resource",
					Source: atc.Source{"some": "source"},
				},
				{
					Name:   "custom-type-c",
					Type:   "custom-type-b",
					Source: atc.Source{"some": "source"},
				},
				{
					Name:   "custom-type-d",
					Type:   "custom-type-b",
					Source: atc.Source{"some": "source"},
				},
				{
					Name:   "unknown-custom-type",
					Type:   "unknown-base-type",
					Source: atc.Source{"some": "source"},
				},
			}
		})

		JustBeforeEach(func() {
			satisfyingWorker, satisfyingErr = gardenWorker.Satisfying(spec, customTypes)
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
				customTypes = append(customTypes, atc.ResourceType{
					Name:   "some-resource",
					Type:   "some-resource",
					Source: atc.Source{"some": "source"},
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
				customTypes = append(customTypes, atc.ResourceType{
					Name:   "circle-a",
					Type:   "circle-b",
					Source: atc.Source{"some": "source"},
				}, atc.ResourceType{
					Name:   "circle-b",
					Type:   "circle-c",
					Source: atc.Source{"some": "source"},
				}, atc.ResourceType{
					Name:   "circle-c",
					Type:   "circle-a",
					Source: atc.Source{"some": "source"},
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
