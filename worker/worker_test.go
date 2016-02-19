package worker_test

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	gfakes "github.com/cloudfoundry-incubator/garden/fakes"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/fakes"
	"github.com/concourse/baggageclaim"
	bfakes "github.com/concourse/baggageclaim/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/clock/fakeclock"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Worker", func() {
	var (
		logger                 *lagertest.TestLogger
		fakeGardenClient       *gfakes.FakeClient
		fakeBaggageclaimClient *bfakes.FakeClient
		fakeVolumeFactory      *wfakes.FakeVolumeFactory
		fakeImageFetcher       *wfakes.FakeImageFetcher
		fakeGardenWorkerDB     *wfakes.FakeGardenWorkerDB
		fakeWorkerProvider     *wfakes.FakeWorkerProvider
		fakeClock              *fakeclock.FakeClock
		activeContainers       int
		resourceTypes          []atc.WorkerResourceType
		platform               string
		tags                   []string
		workerName             string

		gardenWorker Worker
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakeGardenClient = new(gfakes.FakeClient)
		fakeBaggageclaimClient = new(bfakes.FakeClient)
		fakeVolumeFactory = new(wfakes.FakeVolumeFactory)
		fakeImageFetcher = new(wfakes.FakeImageFetcher)
		fakeGardenWorkerDB = new(wfakes.FakeGardenWorkerDB)
		fakeWorkerProvider = new(wfakes.FakeWorkerProvider)
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))
		activeContainers = 42
		resourceTypes = []atc.WorkerResourceType{
			{Type: "some-resource", Image: "some-resource-image"},
		}
		platform = "some-platform"
		tags = []string{"some", "tags"}
		workerName = "some-worker"
	})

	BeforeEach(func() {
		gardenWorker = NewGardenWorker(
			fakeGardenClient,
			fakeBaggageclaimClient,
			fakeVolumeFactory,
			fakeImageFetcher,
			fakeGardenWorkerDB,
			fakeWorkerProvider,
			fakeClock,
			activeContainers,
			resourceTypes,
			platform,
			tags,
			workerName,
		)
	})

	Describe("VolumeManager", func() {
		var baggageclaimClient baggageclaim.Client
		var volumeManager baggageclaim.Client
		var hasVolumeManager bool

		JustBeforeEach(func() {
			volumeManager, hasVolumeManager = NewGardenWorker(
				fakeGardenClient,
				baggageclaimClient,
				fakeVolumeFactory,
				fakeImageFetcher,
				fakeGardenWorkerDB,
				fakeWorkerProvider,
				fakeClock,
				activeContainers,
				resourceTypes,
				platform,
				tags,
				workerName,
			).VolumeManager()
		})

		Context("when there is no baggageclaim client", func() {
			BeforeEach(func() {
				baggageclaimClient = nil
			})

			It("returns nil and false", func() {
				Expect(volumeManager).To(BeNil())
				Expect(hasVolumeManager).To(BeFalse())
			})
		})

		Context("when there is a baggageclaim client", func() {
			BeforeEach(func() {
				baggageclaimClient = new(bfakes.FakeClient)
			})

			It("returns the client and true", func() {
				Expect(volumeManager).To(Equal(baggageclaimClient))
				Expect(hasVolumeManager).To(BeTrue())
			})
		})
	})

	Describe("CreateContainer", func() {
		var (
			logger                    lager.Logger
			signals                   <-chan os.Signal
			fakeImageFetchingDelegate *wfakes.FakeImageFetchingDelegate
			containerID               Identifier
			containerMetadata         Metadata
			spec                      ContainerSpec
			customTypes               atc.ResourceTypes

			createdContainer Container
			createErr        error
		)

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("test")

			signals = make(chan os.Signal)
			fakeImageFetchingDelegate = new(wfakes.FakeImageFetchingDelegate)

			containerID = Identifier{
				BuildID: 42,
			}

			containerMetadata = Metadata{
				BuildName: "lol",
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
			createdContainer, createErr = gardenWorker.CreateContainer(logger, signals, fakeImageFetchingDelegate, containerID, containerMetadata, spec, customTypes)
		})

		Context("with a resource type container spec", func() {
			Context("when the resource type is supported by the worker", func() {
				BeforeEach(func() {
					spec = ResourceTypeContainerSpec{
						Type: "some-resource",
					}
				})

				Context("when creating the garden container works", func() {
					var fakeContainer *gfakes.FakeContainer

					BeforeEach(func() {
						fakeContainer = new(gfakes.FakeContainer)
						fakeContainer.HandleReturns("some-handle")

						fakeGardenClient.CreateReturns(fakeContainer, nil)
					})

					It("succeeds", func() {
						Expect(createErr).NotTo(HaveOccurred())
					})

					It("creates the container with the Garden client", func() {
						Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
						Expect(fakeGardenClient.CreateArgsForCall(0)).To(Equal(garden.ContainerSpec{
							RootFSPath: "some-resource-image",
							Privileged: true,
							Properties: garden.Properties{},
						}))
					})

					It("creates the container info in the database", func() {
						expectedMetadata := containerMetadata
						expectedMetadata.WorkerName = workerName
						expectedMetadata.Handle = "some-handle"

						container := db.Container{
							ContainerIdentifier: db.ContainerIdentifier(containerID),
							ContainerMetadata:   db.ContainerMetadata(expectedMetadata),
						}

						Expect(fakeGardenWorkerDB.CreateContainerCallCount()).To(Equal(1))
						actualContainer, ttl := fakeGardenWorkerDB.CreateContainerArgsForCall(0)
						Expect(actualContainer).To(Equal(container))
						Expect(ttl).To(Equal(5 * time.Minute))
					})

					Context("when creating the container info in the db fails", func() {
						disaster := errors.New("bad")

						BeforeEach(func() {
							fakeGardenWorkerDB.CreateContainerReturns(db.Container{}, disaster)
						})

						It("returns the error", func() {

							Expect(createErr).To(Equal(disaster))
						})

					})

					Context("when an image resource is provided", func() {
						BeforeEach(func() {
							spec = ResourceTypeContainerSpec{
								ImageResourcePointer: &atc.TaskImageConfig{
									Type:   "some-type",
									Source: atc.Source{"some": "source"},
								},
								Env: []string{"C=3"},
							}
						})

						Context("when fetching the image succeeds", func() {
							var image *wfakes.FakeImage

							BeforeEach(func() {
								image = new(wfakes.FakeImage)

								imageVolume := new(bfakes.FakeVolume)
								imageVolume.HandleReturns("image-volume")
								imageVolume.PathReturns("/some/image/path")
								image.VolumeReturns(imageVolume)
								image.MetadataReturns(ImageMetadata{
									Env: []string{"A=1", "B=2"},
								})

								fakeImageFetcher.FetchImageReturns(image, nil)
							})

							It("succeeds", func() {
								Expect(createErr).NotTo(HaveOccurred())
							})

							It("creates the container with (volume path)/rootfs as the rootfs", func() {
								Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))

								spec := fakeGardenClient.CreateArgsForCall(0)
								Expect(spec.RootFSPath).To(Equal("/some/image/path/rootfs"))
								Expect(spec.Properties).To(HaveLen(2))
								Expect(spec.Properties["concourse:volumes"]).To(MatchJSON(
									`["image-volume"]`,
								))
								Expect(spec.Properties["concourse:volume-mounts"]).To(MatchJSON(`{}`))
							})

							It("fetches the image with the correct info", func() {
								Expect(fakeImageFetcher.FetchImageCallCount()).To(Equal(1))
								_, fetchImageConfig, fetchSignals, fetchID, fetchMetadata, fetchDelegate, fetchWorker, fetchCustomTypes := fakeImageFetcher.FetchImageArgsForCall(0)
								Expect(fetchImageConfig).To(Equal(atc.TaskImageConfig{
									Type:   "some-type",
									Source: atc.Source{"some": "source"},
								}))
								Expect(fetchSignals).To(Equal(signals))
								Expect(fetchID).To(Equal(containerID))
								Expect(fetchMetadata).To(Equal(containerMetadata))
								Expect(fetchDelegate).To(Equal(fakeImageFetchingDelegate))
								Expect(fetchWorker).To(Equal(gardenWorker))
								Expect(fetchCustomTypes).To(Equal(customTypes))
							})

							It("creates the container with env from the image", func() {
								Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))

								spec := fakeGardenClient.CreateArgsForCall(0)
								Expect(spec.Env).To(Equal([]string{"A=1", "B=2", "C=3"}))
							})

							Context("after the container is created", func() {
								BeforeEach(func() {
									fakeGardenClient.CreateStub = func(garden.ContainerSpec) (garden.Container, error) {
										Expect(image.ReleaseCallCount()).To(Equal(0))
										return fakeContainer, nil
									}
								})

								It("releases the image", func() {
									Expect(image.ReleaseCallCount()).To(Equal(1))
								})
							})
						})

						Context("when fetching the image fails", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								fakeImageFetcher.FetchImageReturns(nil, disaster)
							})

							It("returns the error", func() {
								Expect(createErr).To(Equal(disaster))
							})
						})
					})

					Context("when env vars are provided", func() {
						BeforeEach(func() {
							spec = ResourceTypeContainerSpec{
								Type: "some-resource",
								Env:  []string{"a=1", "b=2"},
							}
						})

						It("creates the container with the given env vars", func() {
							Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
							Expect(fakeGardenClient.CreateArgsForCall(0)).To(Equal(garden.ContainerSpec{
								RootFSPath: "some-resource-image",
								Privileged: true,
								Env:        []string{"a=1", "b=2"},
								Properties: garden.Properties{},
							}))

						})
					})

					Context("when a volume mount is provided", func() {
						var (
							volume1 *bfakes.FakeVolume
							volume2 *bfakes.FakeVolume
						)

						BeforeEach(func() {
							volume1 = new(bfakes.FakeVolume)
							volume1.HandleReturns("some-volume1")
							volume1.PathReturns("/some/src/path1")

							volume2 = new(bfakes.FakeVolume)
							volume2.HandleReturns("some-volume2")
							volume2.PathReturns("/some/src/path2")
						})

						Context("when copy-on-write is specified", func() {
							var (
								cowVolume1 *bfakes.FakeVolume
								cowVolume2 *bfakes.FakeVolume
							)

							BeforeEach(func() {
								spec = ResourceTypeContainerSpec{
									Type: "some-resource",
									Mounts: []VolumeMount{
										{
											Volume:    volume1,
											MountPath: "/some/dst/path1",
										},
										{
											Volume:    volume2,
											MountPath: "/some/dst/path2",
										},
									},
								}

								cowVolume1 = new(bfakes.FakeVolume)
								cowVolume1.HandleReturns("cow-volume1")
								cowVolume1.PathReturns("/some/cow/src/path")

								cowVolume2 = new(bfakes.FakeVolume)
								cowVolume2.HandleReturns("cow-volume2")
								cowVolume2.PathReturns("/some/other/cow/src/path")

								fakeBaggageclaimClient.CreateVolumeStub = func(logger lager.Logger, spec baggageclaim.VolumeSpec) (baggageclaim.Volume, error) {
									Expect(spec.Privileged).To(BeTrue())
									Expect(spec.TTL).To(Equal(5 * time.Minute))

									if reflect.DeepEqual(spec.Strategy, baggageclaim.COWStrategy{Parent: volume1}) {
										return cowVolume1, nil
									} else if reflect.DeepEqual(spec.Strategy, baggageclaim.COWStrategy{Parent: volume2}) {
										return cowVolume2, nil
									} else {
										Fail(fmt.Sprintf("unknown strategy: %#v", spec.Strategy))
										return nil, nil
									}
								}

								fakeGardenClient.CreateStub = func(garden.ContainerSpec) (garden.Container, error) {
									// ensure they're not released before container creation
									Expect(cowVolume1.ReleaseCallCount()).To(BeZero())
									Expect(cowVolume2.ReleaseCallCount()).To(BeZero())
									Expect(volume1.ReleaseCallCount()).To(BeZero())
									Expect(volume2.ReleaseCallCount()).To(BeZero())
									return fakeContainer, nil
								}
							})

							It("creates the container with read-write copy-on-write bind-mounts for each cache", func() {
								Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))

								spec := fakeGardenClient.CreateArgsForCall(0)
								Expect(spec.RootFSPath).To(Equal("some-resource-image"))
								Expect(spec.Privileged).To(BeTrue())
								Expect(spec.BindMounts).To(Equal([]garden.BindMount{
									{
										SrcPath: "/some/cow/src/path",
										DstPath: "/some/dst/path1",
										Mode:    garden.BindMountModeRW,
									},
									{
										SrcPath: "/some/other/cow/src/path",
										DstPath: "/some/dst/path2",
										Mode:    garden.BindMountModeRW,
									},
								}))

								Expect(spec.Properties).To(HaveLen(2))
								Expect(spec.Properties["concourse:volumes"]).To(MatchJSON(
									`["cow-volume1","cow-volume2"]`,
								))

								Expect(spec.Properties["concourse:volume-mounts"]).To(MatchJSON(
									`{"cow-volume1":"/some/dst/path1","cow-volume2":"/some/dst/path2"}`,
								))
							})

							It("releases the volumes that it instantiated but not the ones that were passed in", func() {
								Expect(cowVolume1.ReleaseCallCount()).To(Equal(1))
								Expect(cowVolume2.ReleaseCallCount()).To(Equal(1))
								Expect(volume1.ReleaseCallCount()).To(BeZero())
								Expect(volume2.ReleaseCallCount()).To(BeZero())
							})

							Context("and the copy-on-write volumes fail to be created", func() {
								disaster := errors.New("par")

								BeforeEach(func() {
									fakeBaggageclaimClient.CreateVolumeReturns(nil, disaster)
								})

								It("errors", func() {
									Expect(createErr).To(Equal(disaster))
									Expect(fakeGardenClient.CreateCallCount()).To(BeZero())
								})
							})
						})

						Context("when a cache is specified", func() {
							BeforeEach(func() {
								spec = ResourceTypeContainerSpec{
									Type: "some-resource",
									Cache: VolumeMount{
										Volume:    volume1,
										MountPath: "/some/dst/path1",
									},
								}
							})

							It("creates the container with a read-write bind-mount", func() {
								Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))

								spec := fakeGardenClient.CreateArgsForCall(0)
								Expect(spec.RootFSPath).To(Equal("some-resource-image"))
								Expect(spec.Privileged).To(BeTrue())
								Expect(spec.BindMounts).To(Equal([]garden.BindMount{
									{
										SrcPath: "/some/src/path1",
										DstPath: "/some/dst/path1",
										Mode:    garden.BindMountModeRW,
									},
								}))

								Expect(spec.Properties).To(HaveLen(2))
								Expect(spec.Properties["concourse:volumes"]).To(MatchJSON(
									`["some-volume1"]`,
								))

								Expect(spec.Properties["concourse:volume-mounts"]).To(MatchJSON(
									`{"some-volume1":"/some/dst/path1"}`,
								))
							})
						})

						Context("when both cache and volumes are specified", func() {
							BeforeEach(func() {
								spec = ResourceTypeContainerSpec{
									Type: "some-resource",
									Cache: VolumeMount{
										Volume:    volume1,
										MountPath: "/some/dst/path1",
									},
									Mounts: []VolumeMount{
										{
											Volume:    volume1,
											MountPath: "/some/dst/path1",
										},
										{
											Volume:    volume2,
											MountPath: "/some/dst/path2",
										},
									},
								}
							})

							It("errors (may be ok but I don't know how they get along)", func() {
								Expect(createErr).To(HaveOccurred())
								Expect(fakeGardenClient.CreateCallCount()).To(BeZero())
							})
						})
					})

					Context("when the container is marked as ephemeral", func() {
						BeforeEach(func() {
							spec = ResourceTypeContainerSpec{
								Type:      "some-resource",
								Ephemeral: true,
							}
						})

						It("adds an 'ephemeral' property to the container", func() {
							Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
							Expect(fakeGardenClient.CreateArgsForCall(0)).To(Equal(garden.ContainerSpec{
								RootFSPath: "some-resource-image",
								Privileged: true,
								Properties: garden.Properties{
									"concourse:ephemeral": "true",
								},
							}))
						})
					})

					Describe("the created container", func() {
						It("can be destroyed", func() {
							err := createdContainer.Destroy()
							Expect(err).NotTo(HaveOccurred())

							By("destroying via garden")
							Expect(fakeGardenClient.DestroyCallCount()).To(Equal(1))
							Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("some-handle"))

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
								Expect(handle).To(Equal("some-handle"))
								Expect(interval).To(Equal(5 * time.Minute))

								fakeClock.Increment(30 * time.Second)

								Eventually(fakeContainer.SetGraceTimeCallCount).Should(Equal(3))
								Expect(fakeContainer.SetGraceTimeArgsForCall(2)).To(Equal(5 * time.Minute))

								Eventually(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount).Should(Equal(3))
								handle, interval = fakeGardenWorkerDB.UpdateExpiresAtOnContainerArgsForCall(2)
								Expect(handle).To(Equal("some-handle"))
								Expect(interval).To(Equal(5 * time.Minute))
							})
						})

						Describe("releasing", func() {
							It("sets a final ttl on the container and stops heartbeating", func() {
								createdContainer.Release(FinalTTL(30 * time.Minute))

								Expect(fakeContainer.SetGraceTimeCallCount()).Should(Equal(2))
								Expect(fakeContainer.SetGraceTimeArgsForCall(1)).To(Equal(30 * time.Minute))

								Expect(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount()).Should(Equal(2))
								handle, interval := fakeGardenWorkerDB.UpdateExpiresAtOnContainerArgsForCall(1)
								Expect(handle).To(Equal("some-handle"))
								Expect(interval).To(Equal(30 * time.Minute))

								fakeClock.Increment(30 * time.Second)

								Consistently(fakeContainer.SetGraceTimeCallCount).Should(Equal(2))
								Consistently(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount).Should(Equal(2))
							})

							Context("with no final ttl", func() {
								It("does not perform a final heartbeat", func() {
									createdContainer.Release(nil)

									Consistently(fakeContainer.SetGraceTimeCallCount).Should(Equal(1))
									Consistently(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount).Should(Equal(1))
								})
							})
						})
					})
				})

				Context("when creating fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeGardenClient.CreateReturns(nil, disaster)
					})

					It("returns the error", func() {
						Expect(createErr).To(Equal(disaster))
					})
				})
			})

			Context("with a custom resource type for a resource container", func() {
				BeforeEach(func() {
					spec = ResourceTypeContainerSpec{
						Type: "custom-type-c",
						Env:  []string{"C=3"},
					}
				})

				Context("when creating the garden container works", func() {
					var fakeContainer *gfakes.FakeContainer

					BeforeEach(func() {
						fakeContainer = new(gfakes.FakeContainer)
						fakeContainer.HandleReturns("some-handle")

						fakeGardenClient.CreateReturns(fakeContainer, nil)
					})

					Context("when fetching the image succeeds", func() {
						var image *wfakes.FakeImage

						BeforeEach(func() {
							image = new(wfakes.FakeImage)

							imageVolume := new(bfakes.FakeVolume)
							imageVolume.HandleReturns("image-volume")
							imageVolume.PathReturns("/some/image/path")
							image.VolumeReturns(imageVolume)
							image.MetadataReturns(ImageMetadata{
								Env: []string{"A=1", "B=2"},
							})

							fakeImageFetcher.FetchImageReturns(image, nil)
						})

						It("succeeds", func() {
							Expect(createErr).NotTo(HaveOccurred())
						})

						It("creates the container with (volume path)/rootfs as the rootfs", func() {
							Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))

							spec := fakeGardenClient.CreateArgsForCall(0)
							Expect(spec.RootFSPath).To(Equal("/some/image/path/rootfs"))
							Expect(spec.Properties).To(HaveLen(2))
							Expect(spec.Properties["concourse:volumes"]).To(MatchJSON(
								`["image-volume"]`,
							))
							Expect(spec.Properties["concourse:volume-mounts"]).To(MatchJSON(`{}`))
						})

						It("fetches the image with the correct info", func() {
							Expect(fakeImageFetcher.FetchImageCallCount()).To(Equal(1))
							_, fetchImageConfig, fetchSignals, fetchID, fetchMetadata, fetchDelegate, fetchWorker, fetchCustomTypes := fakeImageFetcher.FetchImageArgsForCall(0)
							Expect(fetchImageConfig).To(Equal(atc.TaskImageConfig{
								Type:   "custom-type-b",
								Source: atc.Source{"some": "source"},
							}))
							Expect(fetchSignals).To(Equal(signals))
							Expect(fetchID).To(Equal(containerID))
							Expect(fetchMetadata).To(Equal(containerMetadata))
							Expect(fetchDelegate).To(Equal(fakeImageFetchingDelegate))
							Expect(fetchWorker).To(Equal(gardenWorker))
							Expect(fetchCustomTypes).To(Equal(atc.ResourceTypes{
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
								// no c!
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
							}))
						})

						It("creates the container with env from the image", func() {
							Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))

							spec := fakeGardenClient.CreateArgsForCall(0)
							Expect(spec.Env).To(Equal([]string{"A=1", "B=2", "C=3"}))
						})

						Context("after the container is created", func() {
							BeforeEach(func() {
								fakeGardenClient.CreateStub = func(garden.ContainerSpec) (garden.Container, error) {
									Expect(image.ReleaseCallCount()).To(Equal(0))
									return fakeContainer, nil
								}
							})

							It("releases the image", func() {
								Expect(image.ReleaseCallCount()).To(Equal(1))
							})
						})
					})
				})
			})

			Context("with a custom resource type for a task container's image_resource", func() {
				BeforeEach(func() {
					spec = TaskContainerSpec{
						ImageResourcePointer: &atc.TaskImageConfig{
							Type:   "custom-type-c",
							Source: atc.Source{"some": "source"},
						},
					}
				})

				Context("when creating the garden container works", func() {
					var fakeContainer *gfakes.FakeContainer

					BeforeEach(func() {
						fakeContainer = new(gfakes.FakeContainer)
						fakeContainer.HandleReturns("some-handle")

						fakeGardenClient.CreateReturns(fakeContainer, nil)
					})

					Context("when fetching the image succeeds", func() {
						var image *wfakes.FakeImage

						BeforeEach(func() {
							image = new(wfakes.FakeImage)

							imageVolume := new(bfakes.FakeVolume)
							imageVolume.HandleReturns("image-volume")
							imageVolume.PathReturns("/some/image/path")
							image.VolumeReturns(imageVolume)
							image.MetadataReturns(ImageMetadata{
								Env: []string{"A=1", "B=2"},
							})

							fakeImageFetcher.FetchImageReturns(image, nil)
						})

						It("succeeds", func() {
							Expect(createErr).NotTo(HaveOccurred())
						})

						It("creates the container with (volume path)/rootfs as the rootfs", func() {
							Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))

							spec := fakeGardenClient.CreateArgsForCall(0)
							Expect(spec.RootFSPath).To(Equal("/some/image/path/rootfs"))
							Expect(spec.Properties).To(HaveLen(2))
							Expect(spec.Properties["concourse:volumes"]).To(MatchJSON(
								`["image-volume"]`,
							))
							Expect(spec.Properties["concourse:volume-mounts"]).To(MatchJSON(`{}`))
						})

						It("fetches the image with the correct info", func() {
							Expect(fakeImageFetcher.FetchImageCallCount()).To(Equal(1))
							_, fetchImageConfig, fetchSignals, fetchID, fetchMetadata, fetchDelegate, fetchWorker, fetchCustomTypes := fakeImageFetcher.FetchImageArgsForCall(0)
							Expect(fetchImageConfig).To(Equal(atc.TaskImageConfig{
								Type:   "custom-type-c",
								Source: atc.Source{"some": "source"},
							}))
							Expect(fetchSignals).To(Equal(signals))
							Expect(fetchID).To(Equal(containerID))
							Expect(fetchMetadata).To(Equal(containerMetadata))
							Expect(fetchDelegate).To(Equal(fakeImageFetchingDelegate))
							Expect(fetchWorker).To(Equal(gardenWorker))
							Expect(fetchCustomTypes).To(Equal(customTypes))
						})

						Context("after the container is created", func() {
							BeforeEach(func() {
								fakeGardenClient.CreateStub = func(garden.ContainerSpec) (garden.Container, error) {
									Expect(image.ReleaseCallCount()).To(Equal(0))
									return fakeContainer, nil
								}
							})

							It("releases the image", func() {
								Expect(image.ReleaseCallCount()).To(Equal(1))
							})
						})
					})
				})
			})

			Context("when the type is unknown", func() {
				BeforeEach(func() {
					spec = ResourceTypeContainerSpec{
						Type: "some-bogus-resource",
					}
				})

				It("returns ErrUnsupportedResourceType", func() {
					Expect(createErr).To(Equal(ErrUnsupportedResourceType))
				})
			})
		})

		Context("with a task container spec", func() {
			BeforeEach(func() {
				spec = TaskContainerSpec{
					Image:      "some-image",
					Privileged: true,
				}
			})

			Context("when creating works", func() {
				var fakeContainer *gfakes.FakeContainer

				BeforeEach(func() {
					fakeContainer = new(gfakes.FakeContainer)
					fakeContainer.HandleReturns("some-handle")

					fakeGardenClient.CreateReturns(fakeContainer, nil)
				})

				It("succeeds", func() {
					Expect(createErr).NotTo(HaveOccurred())
				})

				It("creates the container with the Garden client", func() {
					Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
					Expect(fakeGardenClient.CreateArgsForCall(0)).To(Equal(garden.ContainerSpec{
						RootFSPath: "some-image",
						Privileged: true,
						Properties: garden.Properties{},
					}))
				})

				Context("when an image resource is provided", func() {
					BeforeEach(func() {
						spec = TaskContainerSpec{
							ImageResourcePointer: &atc.TaskImageConfig{
								Type:   "some-type",
								Source: atc.Source{"some": "source"},
							},
						}
					})

					Context("when fetching the image succeeds", func() {
						var image *wfakes.FakeImage

						BeforeEach(func() {
							image = new(wfakes.FakeImage)

							imageVolume := new(bfakes.FakeVolume)
							imageVolume.HandleReturns("image-volume")
							imageVolume.PathReturns("/some/image/path")
							image.VolumeReturns(imageVolume)
							image.MetadataReturns(ImageMetadata{
								Env:  []string{"A=1", "B=2"},
								User: "pilot",
							})

							fakeImageFetcher.FetchImageReturns(image, nil)
						})

						It("succeeds", func() {
							Expect(createErr).NotTo(HaveOccurred())
						})

						It("creates the container with (volume path)/rootfs as the rootfs", func() {
							Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))

							spec := fakeGardenClient.CreateArgsForCall(0)
							Expect(spec.RootFSPath).To(Equal("/some/image/path/rootfs"))
							Expect(spec.Properties).To(HaveLen(3))
							Expect(spec.Properties["concourse:volumes"]).To(MatchJSON(
								`["image-volume"]`,
							))
							Expect(spec.Properties["concourse:volume-mounts"]).To(MatchJSON(`{}`))
						})

						It("fetches the image with the correct info", func() {
							Expect(fakeImageFetcher.FetchImageCallCount()).To(Equal(1))
							_, fetchImageConfig, fetchSignals, fetchID, fetchMetadata, fetchDelegate, fetchWorker, fetchCustomTypes := fakeImageFetcher.FetchImageArgsForCall(0)
							Expect(fetchImageConfig).To(Equal(atc.TaskImageConfig{
								Type:   "some-type",
								Source: atc.Source{"some": "source"},
							}))
							Expect(fetchSignals).To(Equal(signals))
							Expect(fetchID).To(Equal(containerID))
							Expect(fetchMetadata).To(Equal(containerMetadata))
							Expect(fetchDelegate).To(Equal(fakeImageFetchingDelegate))
							Expect(fetchWorker).To(Equal(gardenWorker))
							Expect(fetchCustomTypes).To(Equal(customTypes))
						})

						It("creates the container with env from the image", func() {
							Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))

							spec := fakeGardenClient.CreateArgsForCall(0)
							Expect(spec.Env).To(Equal([]string{"A=1", "B=2"}))
						})

						It("creates the container info in the database", func() {
							expectedMetadata := containerMetadata
							expectedMetadata.WorkerName = workerName
							expectedMetadata.Handle = "some-handle"
							expectedMetadata.User = "pilot"

							container := db.Container{
								ContainerIdentifier: db.ContainerIdentifier(containerID),
								ContainerMetadata:   db.ContainerMetadata(expectedMetadata),
							}

							Expect(fakeGardenWorkerDB.CreateContainerCallCount()).To(Equal(1))
							actualContainer, ttl := fakeGardenWorkerDB.CreateContainerArgsForCall(0)
							Expect(actualContainer).To(Equal(container))
							Expect(ttl).To(Equal(5 * time.Minute))
						})

						Context("after the container is created", func() {
							BeforeEach(func() {
								fakeGardenClient.CreateStub = func(garden.ContainerSpec) (garden.Container, error) {
									Expect(image.ReleaseCallCount()).To(Equal(0))
									return fakeContainer, nil
								}
							})

							It("releases the image", func() {
								Expect(image.ReleaseCallCount()).To(Equal(1))
							})
						})
					})

					Context("when fetching the image fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeImageFetcher.FetchImageReturns(nil, disaster)
						})

						It("returns the error", func() {
							Expect(createErr).To(Equal(disaster))
						})
					})
				})

				Context("when outputs are provided", func() {
					var volume1 *bfakes.FakeVolume
					var volume2 *bfakes.FakeVolume

					var taskSpec TaskContainerSpec

					BeforeEach(func() {
						volume1 = new(bfakes.FakeVolume)
						volume1.HandleReturns("output-volume")
						volume1.PathReturns("/some/src/path")

						volume2 = new(bfakes.FakeVolume)
						volume2.HandleReturns("other-output-volume")
						volume2.PathReturns("/some/other/src/path")

						taskSpec = spec.(TaskContainerSpec)

						taskSpec.Outputs = []VolumeMount{
							{
								Volume:    volume1,
								MountPath: "/tmp/dst/path",
							},
							{
								Volume:    volume2,
								MountPath: "/tmp/dst/other/path",
							},
						}

						spec = taskSpec
					})

					It("creates the container with read-write bind-mounts for each output", func() {
						Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))

						spec := fakeGardenClient.CreateArgsForCall(0)
						Expect(spec.RootFSPath).To(Equal("some-image"))
						Expect(spec.Privileged).To(BeTrue())
						Expect(spec.BindMounts).To(Equal([]garden.BindMount{
							{
								SrcPath: "/some/src/path",
								DstPath: "/tmp/dst/path",
								Mode:    garden.BindMountModeRW,
							},
							{
								SrcPath: "/some/other/src/path",
								DstPath: "/tmp/dst/other/path",
								Mode:    garden.BindMountModeRW,
							},
						}))

						Expect(spec.Properties).To(HaveLen(2))
						Expect(spec.Properties["concourse:volumes"]).To(MatchJSON(
							`["output-volume","other-output-volume"]`,
						))

						Expect(spec.Properties["concourse:volume-mounts"]).To(MatchJSON(
							`{"output-volume":"/tmp/dst/path","other-output-volume":"/tmp/dst/other/path"}`,
						))
					})
				})

				Context("when inputs are provided", func() {
					var volume1 *bfakes.FakeVolume
					var volume2 *bfakes.FakeVolume

					var cowInputVolume *bfakes.FakeVolume
					var cowOtherInputVolume *bfakes.FakeVolume

					var taskSpec TaskContainerSpec

					BeforeEach(func() {
						volume1 = new(bfakes.FakeVolume)
						volume1.HandleReturns("some-volume")
						volume1.PathReturns("/some/src/path")

						volume2 = new(bfakes.FakeVolume)
						volume2.HandleReturns("some-other-volume")
						volume2.PathReturns("/some/other/src/path")

						cowInputVolume = new(bfakes.FakeVolume)
						cowInputVolume.HandleReturns("cow-input-volume")
						cowInputVolume.PathReturns("/some/cow/src/path")

						cowOtherInputVolume = new(bfakes.FakeVolume)
						cowOtherInputVolume.HandleReturns("cow-other-input-volume")
						cowOtherInputVolume.PathReturns("/some/other/cow/src/path")

						fakeBaggageclaimClient.CreateVolumeStub = func(logger lager.Logger, spec baggageclaim.VolumeSpec) (baggageclaim.Volume, error) {
							Expect(spec.Privileged).To(BeTrue())
							Expect(spec.TTL).To(Equal(5 * time.Minute))

							if reflect.DeepEqual(spec.Strategy, baggageclaim.COWStrategy{Parent: volume1}) {
								return cowInputVolume, nil
							} else if reflect.DeepEqual(spec.Strategy, baggageclaim.COWStrategy{Parent: volume2}) {
								return cowOtherInputVolume, nil
							} else {
								Fail(fmt.Sprintf("unknown strategy: %#v", spec.Strategy))
								return nil, nil
							}
						}

						taskSpec = spec.(TaskContainerSpec)

						taskSpec.Inputs = []VolumeMount{
							{
								Volume:    volume1,
								MountPath: "/tmp/dst/path",
							},
							{
								Volume:    volume2,
								MountPath: "/tmp/dst/other/path",
							},
						}

						spec = taskSpec
					})

					It("creates the container with read-write copy-on-write bind-mounts for each input", func() {
						Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))

						spec := fakeGardenClient.CreateArgsForCall(0)
						Expect(spec.RootFSPath).To(Equal("some-image"))
						Expect(spec.Privileged).To(BeTrue())
						Expect(spec.BindMounts).To(Equal([]garden.BindMount{
							{
								SrcPath: "/some/cow/src/path",
								DstPath: "/tmp/dst/path",
								Mode:    garden.BindMountModeRW,
							},
							{
								SrcPath: "/some/other/cow/src/path",
								DstPath: "/tmp/dst/other/path",
								Mode:    garden.BindMountModeRW,
							},
						}))

						Expect(spec.Properties).To(HaveLen(2))
						Expect(spec.Properties["concourse:volumes"]).To(MatchJSON(
							`["cow-input-volume","cow-other-input-volume"]`,
						))

						Expect(spec.Properties["concourse:volume-mounts"]).To(MatchJSON(
							`{"cow-input-volume":"/tmp/dst/path","cow-other-input-volume":"/tmp/dst/other/path"}`,
						))
					})

					Context("after the container is created", func() {
						BeforeEach(func() {
							fakeGardenClient.CreateStub = func(garden.ContainerSpec) (garden.Container, error) {
								// ensure they're not released before container creation
								Expect(cowInputVolume.ReleaseCallCount()).To(Equal(0))
								Expect(cowOtherInputVolume.ReleaseCallCount()).To(Equal(0))
								return fakeContainer, nil
							}
						})

						It("releases the copy-on-write volumes that it made beforehand", func() {
							Expect(cowInputVolume.ReleaseCallCount()).To(Equal(1))
							Expect(cowOtherInputVolume.ReleaseCallCount()).To(Equal(1))
						})
					})

					Context("when creating the copy-on-write volumes fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeBaggageclaimClient.CreateVolumeReturns(nil, disaster)
						})

						It("returns the error", func() {
							Expect(createErr).To(Equal(disaster))
						})
					})
				})

				Describe("the created container", func() {
					It("can be destroyed", func() {
						err := createdContainer.Destroy()
						Expect(err).NotTo(HaveOccurred())

						By("destroying via garden")
						Expect(fakeGardenClient.DestroyCallCount()).To(Equal(1))
						Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("some-handle"))

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
							Expect(handle).To(Equal("some-handle"))
							Expect(interval).To(Equal(5 * time.Minute))

							fakeClock.Increment(30 * time.Second)

							Eventually(fakeContainer.SetGraceTimeCallCount).Should(Equal(3))
							Expect(fakeContainer.SetGraceTimeArgsForCall(2)).To(Equal(5 * time.Minute))

							Eventually(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount).Should(Equal(3))
							handle, interval = fakeGardenWorkerDB.UpdateExpiresAtOnContainerArgsForCall(2)
							Expect(handle).To(Equal("some-handle"))
							Expect(interval).To(Equal(5 * time.Minute))
						})
					})

					Describe("releasing", func() {
						It("sets a final ttl on the container and stops heartbeating", func() {
							createdContainer.Release(FinalTTL(30 * time.Minute))

							Expect(fakeContainer.SetGraceTimeCallCount()).Should(Equal(2))
							Expect(fakeContainer.SetGraceTimeArgsForCall(1)).To(Equal(30 * time.Minute))

							Expect(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount()).Should(Equal(2))
							handle, interval := fakeGardenWorkerDB.UpdateExpiresAtOnContainerArgsForCall(1)
							Expect(handle).To(Equal("some-handle"))
							Expect(interval).To(Equal(30 * time.Minute))

							fakeClock.Increment(30 * time.Second)

							Consistently(fakeContainer.SetGraceTimeCallCount).Should(Equal(2))
							Consistently(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount).Should(Equal(2))
						})

						Context("with no final ttl", func() {
							It("does not perform a final heartbeat", func() {
								createdContainer.Release(nil)

								Consistently(fakeContainer.SetGraceTimeCallCount).Should(Equal(1))
								Consistently(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount).Should(Equal(1))
							})
						})
					})
				})
			})

			Context("when creating fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeGardenClient.CreateReturns(nil, disaster)
				})

				It("returns the error", func() {
					Expect(createErr).To(Equal(disaster))
				})
			})
		})
	})

	Describe("LookupContainer", func() {
		var handle string

		BeforeEach(func() {
			handle = "we98lsv"
		})

		Context("when the gardenClient returns a container and no error", func() {
			var (
				fakeContainer *gfakes.FakeContainer
			)

			BeforeEach(func() {
				fakeContainer = new(gfakes.FakeContainer)
				fakeContainer.HandleReturns("some-handle")

				fakeGardenClient.LookupReturns(fakeContainer, nil)
			})

			It("returns the container and no error", func() {
				foundContainer, found, err := gardenWorker.LookupContainer(logger, handle)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(foundContainer.Handle()).To(Equal(fakeContainer.Handle()))
			})

			Describe("the container", func() {
				var foundContainer Container
				var findErr error

				JustBeforeEach(func() {
					foundContainer, _, findErr = gardenWorker.LookupContainer(logger, handle)
				})

				Context("when the concourse:volumes property is present", func() {
					var (
						handle1Volume         *bfakes.FakeVolume
						handle2Volume         *bfakes.FakeVolume
						expectedHandle1Volume *wfakes.FakeVolume
						expectedHandle2Volume *wfakes.FakeVolume
					)

					BeforeEach(func() {
						handle1Volume = new(bfakes.FakeVolume)
						handle2Volume = new(bfakes.FakeVolume)
						expectedHandle1Volume = new(wfakes.FakeVolume)
						expectedHandle2Volume = new(wfakes.FakeVolume)

						fakeContainer.PropertiesReturns(garden.Properties{
							"concourse:volumes":       `["handle-1","handle-2"]`,
							"concourse:volume-mounts": `{"handle-1":"/handle-1/path","handle-2":"/handle-2/path"}`,
						}, nil)

						fakeBaggageclaimClient.LookupVolumeStub = func(logger lager.Logger, handle string) (baggageclaim.Volume, bool, error) {
							if handle == "handle-1" {
								return handle1Volume, true, nil
							} else if handle == "handle-2" {
								return handle2Volume, true, nil
							} else {
								panic("unknown handle: " + handle)
							}
						}

						fakeVolumeFactory.BuildStub = func(logger lager.Logger, vol baggageclaim.Volume) (Volume, error) {
							if vol == handle1Volume {
								return expectedHandle1Volume, nil
							} else if vol == handle2Volume {
								return expectedHandle2Volume, nil
							} else {
								panic("unknown volume: " + vol.Handle())
							}
						}
					})

					Describe("Volumes", func() {
						It("returns all bound volumes based on properties on the container", func() {
							Expect(foundContainer.Volumes()).To(Equal([]Volume{
								expectedHandle1Volume,
								expectedHandle2Volume,
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

						Context("when LookupVolume cannot find the volume", func() {
							BeforeEach(func() {
								fakeBaggageclaimClient.LookupVolumeReturns(nil, false, nil)
							})

							It("returns ErrMissingVolume", func() {
								Expect(findErr).To(Equal(ErrMissingVolume))
							})
						})

						Context("when Build returns an error", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								fakeVolumeFactory.BuildReturns(nil, disaster)
							})

							It("returns the error on lookup", func() {
								Expect(findErr).To(Equal(disaster))
							})
						})

						Context("when there is no baggageclaim", func() {
							BeforeEach(func() {
								gardenWorker = NewGardenWorker(
									fakeGardenClient,
									nil,
									nil,
									fakeImageFetcher,
									fakeGardenWorkerDB,
									fakeWorkerProvider,
									fakeClock,
									activeContainers,
									resourceTypes,
									platform,
									tags,
									workerName,
								)
							})

							It("returns an empty slice", func() {
								Expect(foundContainer.Volumes()).To(BeEmpty())
							})
						})
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

						Context("when Build returns an error", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								fakeVolumeFactory.BuildReturns(nil, disaster)
							})

							It("returns the error on lookup", func() {
								Expect(findErr).To(Equal(disaster))
							})
						})

						Context("when there is no baggageclaim", func() {
							BeforeEach(func() {
								gardenWorker = NewGardenWorker(
									fakeGardenClient,
									nil,
									nil,
									fakeImageFetcher,
									fakeGardenWorkerDB,
									fakeWorkerProvider,
									fakeClock,
									activeContainers,
									resourceTypes,
									platform,
									tags,
									workerName,
								)
							})

							It("returns an empty slice", func() {
								Expect(foundContainer.Volumes()).To(BeEmpty())
							})
						})
					})

					Describe("Release", func() {
						It("releases the container's volumes once and only once", func() {
							foundContainer.Release(FinalTTL(time.Minute))
							Expect(expectedHandle1Volume.ReleaseCallCount()).To(Equal(1))
							Expect(expectedHandle1Volume.ReleaseArgsForCall(0)).To(Equal(FinalTTL(time.Minute)))
							Expect(expectedHandle2Volume.ReleaseCallCount()).To(Equal(1))
							Expect(expectedHandle2Volume.ReleaseArgsForCall(0)).To(Equal(FinalTTL(time.Minute)))

							foundContainer.Release(FinalTTL(time.Hour))
							Expect(expectedHandle1Volume.ReleaseCallCount()).To(Equal(1))
							Expect(expectedHandle2Volume.ReleaseCallCount()).To(Equal(1))
						})
					})
				})

				Context("when the concourse:volumes property is not present", func() {
					BeforeEach(func() {
						fakeContainer.PropertiesReturns(garden.Properties{}, nil)
					})

					Describe("Volumes", func() {
						It("returns an empty slice", func() {
							Expect(foundContainer.Volumes()).To(BeEmpty())
						})
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
				fakeContainer *gfakes.FakeContainer
			)

			BeforeEach(func() {
				fakeContainer = new(gfakes.FakeContainer)
				fakeContainer.HandleReturns("provider-handle")

				fakeWorkerProvider.FindContainerForIdentifierReturns(db.Container{
					ContainerMetadata: db.ContainerMetadata{
						Handle: "provider-handle",
					},
				}, true, nil)

				fakeGardenClient.LookupReturns(fakeContainer, nil)
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

		Context("when no containers are found", func() {
			BeforeEach(func() {
				fakeWorkerProvider.FindContainerForIdentifierReturns(db.Container{}, false, nil)
			})

			It("returns that the container could not be found", func() {
				Expect(found).To(BeFalse())
			})
		})

		Context("when finding the containers fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeWorkerProvider.FindContainerForIdentifierReturns(db.Container{}, false, disaster)
			})

			It("returns the error", func() {
				Expect(lookupErr).To(Equal(disaster))
			})
		})

		Context("when the container cannot be found", func() {
			BeforeEach(func() {
				containerToReturn := db.Container{
					ContainerMetadata: db.ContainerMetadata{
						Handle: "handle",
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
				containerToReturn := db.Container{
					ContainerMetadata: db.ContainerMetadata{
						Handle: "handle",
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
			spec = WorkerSpec{}
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
			gardenWorker = NewGardenWorker(
				fakeGardenClient,
				fakeBaggageclaimClient,
				fakeVolumeFactory,
				fakeImageFetcher,
				fakeGardenWorkerDB,
				fakeWorkerProvider,
				fakeClock,
				activeContainers,
				resourceTypes,
				platform,
				tags,
				workerName,
			)

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
				spec.Tags = []string{"some", "tags"}
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
				spec.Tags = []string{"some", "tags"}
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
				spec.Tags = []string{"some", "tags"}
			})

			It("returns ErrUnsupportedResourceType", func() {
				Expect(satisfyingErr).To(Equal(ErrUnsupportedResourceType))
			})
		})

		Context("when the resource type is a custom type not supported by the worker", func() {
			BeforeEach(func() {
				spec.ResourceType = "unknown-custom-type"
				spec.Tags = []string{"some", "tags"}
			})

			It("returns ErrUnsupportedResourceType", func() {
				Expect(satisfyingErr).To(Equal(ErrUnsupportedResourceType))
			})
		})

		Context("when the type is not supported by the worker", func() {
			BeforeEach(func() {
				spec.ResourceType = "some-other-resource"
			})

			Context("when all of the requested tags are present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"some", "tags"}
				})

				It("returns ErrUnsupportedResourceType", func() {
					Expect(satisfyingErr).To(Equal(ErrUnsupportedResourceType))
				})
			})

			Context("when some of the requested tags are present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"some"}
				})

				It("returns ErrUnsupportedResourceType", func() {
					Expect(satisfyingErr).To(Equal(ErrUnsupportedResourceType))
				})
			})

			Context("when any of the requested tags are not present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"bogus", "tags"}
				})

				It("returns ErrUnsupportedResourceType", func() {
					Expect(satisfyingErr).To(Equal(ErrUnsupportedResourceType))
				})
			})
		})
	})
})
