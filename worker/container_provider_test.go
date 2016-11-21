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
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/dbng/dbngfakes"
	. "github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/workerfakes"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/baggageclaimfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContainerProvider", func() {
	var (
		logger                    *lagertest.TestLogger
		fakeImageFetchingDelegate *wfakes.FakeImageFetchingDelegate

		fakeCreatingContainer *dbng.CreatingContainer
		fakeCreatedContainer  *dbng.CreatedContainer

		fakeGardenClient       *gfakes.FakeClient
		fakeBaggageclaimClient *baggageclaimfakes.FakeClient
		fakeVolumeClient       *wfakes.FakeVolumeClient
		fakeImageFactory       *wfakes.FakeImageFactory
		fakeImage              *wfakes.FakeImage
		fakeDBContainerFactory *wfakes.FakeDBContainerFactory
		fakeDBVolumeFactory    *dbngfakes.FakeVolumeFactory
		fakeGardenWorkerDB     *wfakes.FakeGardenWorkerDB
		fakeClock              *fakeclock.FakeClock
		fakeWorker             *wfakes.FakeWorker

		containerProvider        ContainerProvider
		containerProviderFactory ContainerProviderFactory
		outputPaths              map[string]string
		inputs                   []VolumeMount
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		inputs = []VolumeMount{}

		fakeCreatingContainer = &dbng.CreatingContainer{ID: 42}
		fakeCreatedContainer = &dbng.CreatedContainer{ID: 42}

		fakeImageFetchingDelegate = new(wfakes.FakeImageFetchingDelegate)

		fakeGardenClient = new(gfakes.FakeClient)
		fakeBaggageclaimClient = new(baggageclaimfakes.FakeClient)
		fakeVolumeClient = new(wfakes.FakeVolumeClient)
		fakeImageFactory = new(wfakes.FakeImageFactory)
		fakeImage = new(wfakes.FakeImage)
		fakeImageFactory.NewImageReturns(fakeImage)
		fakeGardenWorkerDB = new(wfakes.FakeGardenWorkerDB)
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))
		fakeWorker = new(wfakes.FakeWorker)

		fakeDBContainerFactory = new(wfakes.FakeDBContainerFactory)
		fakeDBVolumeFactory = new(dbngfakes.FakeVolumeFactory)

		containerProviderFactory = NewContainerProviderFactory(
			fakeGardenClient,
			fakeBaggageclaimClient,
			fakeVolumeClient,
			fakeImageFactory,
			fakeDBContainerFactory,
			fakeDBVolumeFactory,
			fakeGardenWorkerDB,
			fakeClock)

		containerProvider = containerProviderFactory.ContainerProviderFor(fakeWorker)
		outputPaths = map[string]string{}
	})

	Describe("FindOrCreateContainer", func() {
		var (
			container Container
			err       error
			imageSpec ImageSpec
		)

		JustBeforeEach(func() {
			container, err = containerProvider.FindOrCreateContainer(
				logger,
				nil,
				fakeCreatingContainer,
				fakeImageFetchingDelegate,
				Identifier{},
				Metadata{},
				ContainerSpec{
					ImageSpec: imageSpec,
					Inputs:    inputs,
				},
				atc.ResourceTypes{
					{
						Type:   "some-resource",
						Name:   "custom-type-b",
						Source: atc.Source{"some": "source"},
					},
				},
				outputPaths,
			)
		})

		XContext("there is an existing container matching", func() {

		})

		Context("a new container is needed", func() {

			var fakeGardenContainer *gfakes.FakeContainer

			BeforeEach(func() {
				fakeDBContainerFactory.ContainerCreatedReturns(fakeCreatedContainer, nil)
				fakeGardenContainer = new(gfakes.FakeContainer)
				fakeGardenContainer.HandleReturns("some-handle")
				fakeGardenClient.CreateReturns(fakeGardenContainer, nil)
			})

			It("returns the newly created container", func() {
				Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
				Expect(container.Handle()).To(Equal("some-handle"))
			})

			Context("when output paths are specified", func() {
				var (
					fakeOutputVolume *wfakes.FakeVolume
				)

				BeforeEach(func() {
					outputPaths = map[string]string{"output": "/some/path"}
					fakeOutputVolume = new(wfakes.FakeVolume)
					fakeOutputVolume.HandleReturns("output-volume-handle")
					fakeVolumeClient.FindOrCreateVolumeForContainerReturns(fakeOutputVolume, nil)
				})

				It("finds or creates the volume using the volume client", func ()  {
					Expect(fakeVolumeClient.FindOrCreateVolumeForContainerCallCount()).To(Equal(1))
					_, spec, _, _, outputPath := fakeVolumeClient.FindOrCreateVolumeForContainerArgsForCall(0)
					s, ok := spec.Strategy.(OutputStrategy)
					Expect(ok).To(BeTrue())
					Expect(s.Name).To(Equal("output"))
					Expect(outputPath).To(Equal("/some/path"))
				})

				Context("when finding / creating the output volume fails", func() {
					var focVolumeErr     error

					BeforeEach(func() {
						focVolumeErr = errors.New("oh noes")
						fakeVolumeClient.FindOrCreateVolumeForContainerReturns(fakeOutputVolume, focVolumeErr)
					})

					It("returns the error", func() {
						Expect(err).To(Equal(focVolumeErr))
					})

				})

			})

			Context("when inputs are specified on the container spec", func() {
				var fakeInputVolume *wfakes.FakeVolume

				BeforeEach(func()  {
						fakeInputVolume = new(wfakes.FakeVolume)
						fakeInputVolume.PathReturns("/some/volume/path")
						inputs = []VolumeMount{
							VolumeMount{
								Volume : fakeInputVolume,
								MountPath : "/some/input/path",
							},
						}

						fakeCOWVolume := new(wfakes.FakeVolume)
						fakeCOWVolume.PathReturns("/some/volume/path")
						fakeVolumeClient.FindOrCreateVolumeForContainerReturns(fakeCOWVolume, nil)
				})

				It("finds / creates COW volumes from the inputs", func ()  {
							Expect(fakeVolumeClient.FindOrCreateVolumeForContainerCallCount()).To(Equal(1))
							_, spec, _, _, mountPath := fakeVolumeClient.FindOrCreateVolumeForContainerArgsForCall(0)
							s, ok := spec.Strategy.(ContainerRootFSStrategy)
							Expect(ok).To(BeTrue())
							Expect(s.Parent).To(Equal(fakeInputVolume))
							Expect(mountPath).To(Equal("/some/input/path"))
				})

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
							Expect(imageArtifactSource.VolumeOnArgsForCall(0)).To(Equal(fakeWorker))
						})

						Context("when streaming the artifact source to the volume fails", func() {
							var disaster error
							BeforeEach(func() {
								disaster = errors.New("this is bad")
								imageArtifactSource.StreamToReturns(disaster)
							})

							It("returns the error", func() {
								Expect(err).To(Equal(disaster))
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
									Expect(err).To(Equal(disaster))
								})
							})

							Context("when streaming the metadata from the worker succeeds", func() {
								It("creates container with image volume and metadata", func() {
									Expect(err).ToNot(HaveOccurred())

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
							Expect(imageArtifactSource.VolumeOnArgsForCall(0)).To(Equal(fakeWorker))
						})

						Context("when streaming the metadata from the worker fails", func() {
							var disaster error
							BeforeEach(func() {
								disaster = errors.New("got em")
								imageArtifactSource.StreamFileReturns(nil, disaster)
							})

							It("returns the error", func() {
								Expect(err).To(Equal(disaster))
							})
						})

						Context("when streaming the metadata from the worker succeeds", func() {
							BeforeEach(func() {
								imageArtifactSource.StreamFileReturns(metadataReader, nil)
							})

							It("creates container with image volume and metadata", func() {
								Expect(err).ToNot(HaveOccurred())

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
						resourceTypes := []atc.WorkerResourceType{
							{
								Type:    "some-resource",
								Image:   "some-resource-image",
								Version: "some-version",
							},
						}
						fakeWorker.ResourceTypesReturns(resourceTypes)
					})

					It("creates container with base resource type volume", func() {
						Expect(err).ToNot(HaveOccurred())
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

	})

	Describe("FindContainerByHandle", func() {
		var (
			foundContainer Container
			findErr        error
			found          bool
		)

		JustBeforeEach(func() {
			foundContainer, found, findErr = containerProvider.FindContainerByHandle(logger, "some-container-handle")
		})

		Context("when the gardenClient returns a container and no error", func() {
			var (
				fakeContainer *gfakes.FakeContainer
			)

			BeforeEach(func() {
				fakeContainer = new(gfakes.FakeContainer)
				fakeContainer.HandleReturns("provider-handle")

				fakeDBVolumeFactory.FindVolumesForContainerReturns([]dbng.CreatedVolume{}, nil)

				fakeDBContainerFactory.FindContainerReturns(&dbng.CreatedContainer{}, true, nil)
				fakeGardenClient.LookupReturns(fakeContainer, nil)
			})

			It("returns the container", func() {
				Expect(findErr).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(foundContainer.Handle()).To(Equal(fakeContainer.Handle()))
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
				Expect(findErr).ToNot(HaveOccurred())
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
				Expect(findErr).To(Equal(expectedErr))

				Expect(foundContainer).To(BeNil())
			})
		})

	})

})
