package worker_test

import (
	"errors"
	"fmt"
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
		fakeGardenWorkerDB     *wfakes.FakeGardenWorkerDB
		fakeWorkerProvider     *wfakes.FakeWorkerProvider
		fakeClock              *fakeclock.FakeClock
		activeContainers       int
		resourceTypes          []atc.WorkerResourceType
		platform               string
		tags                   []string
		name                   string

		worker Worker
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakeGardenClient = new(gfakes.FakeClient)
		fakeBaggageclaimClient = new(bfakes.FakeClient)
		fakeGardenWorkerDB = new(wfakes.FakeGardenWorkerDB)
		fakeWorkerProvider = new(wfakes.FakeWorkerProvider)
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))
		activeContainers = 42
		resourceTypes = []atc.WorkerResourceType{
			{Type: "some-resource", Image: "some-resource-image"},
		}
		platform = "some-platform"
		tags = []string{"some", "tags"}
		name = "my-garden-worker"
	})

	BeforeEach(func() {
		worker = NewGardenWorker(
			fakeGardenClient,
			fakeBaggageclaimClient,
			fakeGardenWorkerDB,
			fakeWorkerProvider,
			fakeClock,
			activeContainers,
			resourceTypes,
			platform,
			tags,
			name,
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
				fakeGardenWorkerDB,
				fakeWorkerProvider,
				fakeClock,
				activeContainers,
				resourceTypes,
				platform,
				tags,
				name,
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
			logger lager.Logger
			id     Identifier
			spec   ContainerSpec

			createdContainer Container
			createErr        error
		)

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("test")

			id = Identifier{
				ContainerIdentifier: db.ContainerIdentifier{
					Name:         "some-name",
					PipelineName: "some-pipeline",
					BuildID:      42,
					Type:         db.ContainerTypeGet,
					CheckType:    "some-check-type",
					CheckSource:  atc.Source{"some": "source"},
				},
				StepLocation: 3,
			}
		})

		JustBeforeEach(func() {
			createdContainer, createErr = worker.CreateContainer(logger, id, spec)
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
							Properties: garden.Properties{
								"concourse:type":          "get",
								"concourse:pipeline-name": "some-pipeline",
								"concourse:location":      "3",
								"concourse:check-type":    "some-check-type",
								"concourse:check-source":  "{\"some\":\"source\"}",
								"concourse:name":          "some-name",
								"concourse:build-id":      "42",
							},
						}))

					})

					It("creates the container info in the database", func() {
						containerInfo := db.ContainerInfo{
							Handle: "some-handle",
							ContainerIdentifier: db.ContainerIdentifier{
								Name:         "some-name",
								PipelineName: "some-pipeline",
								BuildID:      42,
								Type:         db.ContainerTypeGet,
								WorkerName:   "my-garden-worker",
								CheckType:    "some-check-type",
								CheckSource:  atc.Source{"some": "source"},
							},
						}

						Expect(fakeGardenWorkerDB.CreateContainerInfoCallCount()).To(Equal(1))
						actualContainerInfo, ttl := fakeGardenWorkerDB.CreateContainerInfoArgsForCall(0)
						Expect(actualContainerInfo).To(Equal(containerInfo))
						Expect(ttl).To(Equal(5 * time.Minute))
					})

					Context("when creating the container info in the db fails", func() {
						disaster := errors.New("bad")

						BeforeEach(func() {
							fakeGardenWorkerDB.CreateContainerInfoReturns(disaster)
						})

						It("returns the error", func() {

							Expect(createErr).To(Equal(disaster))
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
								Properties: garden.Properties{
									"concourse:type":          "get",
									"concourse:pipeline-name": "some-pipeline",
									"concourse:location":      "3",
									"concourse:check-type":    "some-check-type",
									"concourse:check-source":  "{\"some\":\"source\"}",
									"concourse:name":          "some-name",
									"concourse:build-id":      "42",
								},
								Env: []string{"a=1", "b=2"},
							}))

						})
					})

					Context("when a volume mount is provided", func() {
						var volume *bfakes.FakeVolume

						BeforeEach(func() {
							volume = new(bfakes.FakeVolume)
							volume.HandleReturns("some-volume")
							volume.PathReturns("/some/src/path")

							spec = ResourceTypeContainerSpec{
								Type: "some-resource",
								Cache: VolumeMount{
									Volume:    volume,
									MountPath: "/some/dst/path",
								},
							}
						})

						It("creates the container with a read-write bind-mount", func() {
							Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
							Expect(fakeGardenClient.CreateArgsForCall(0)).To(Equal(garden.ContainerSpec{
								RootFSPath: "some-resource-image",
								Privileged: true,
								Properties: garden.Properties{
									"concourse:type":          "get",
									"concourse:pipeline-name": "some-pipeline",
									"concourse:location":      "3",
									"concourse:check-type":    "some-check-type",
									"concourse:check-source":  `{"some":"source"}`,
									"concourse:name":          "some-name",
									"concourse:build-id":      "42",
									"concourse:volumes":       `["some-volume"]`,
								},
								BindMounts: []garden.BindMount{
									{
										SrcPath: "/some/src/path",
										DstPath: "/some/dst/path",
										Mode:    garden.BindMountModeRW,
									},
								},
							}))

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
									"concourse:type":          "get",
									"concourse:pipeline-name": "some-pipeline",
									"concourse:location":      "3",
									"concourse:check-type":    "some-check-type",
									"concourse:check-source":  "{\"some\":\"source\"}",
									"concourse:name":          "some-name",
									"concourse:build-id":      "42",
									"concourse:ephemeral":     "true",
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
							Consistently(fakeContainer.SetPropertyCallCount).Should(BeZero())
						})

						It("is kept alive by continuously setting a keepalive property until released", func() {
							Expect(fakeContainer.SetPropertyCallCount()).To(Equal(0))

							fakeClock.Increment(30 * time.Second)

							Eventually(fakeContainer.SetPropertyCallCount).Should(Equal(1))
							name, value := fakeContainer.SetPropertyArgsForCall(0)
							Expect(name).To(Equal("keepalive"))
							Expect(value).To(Equal("153")) // unix timestamp

							fakeClock.Increment(30 * time.Second)

							Eventually(fakeContainer.SetPropertyCallCount).Should(Equal(2))
							name, value = fakeContainer.SetPropertyArgsForCall(1)
							Expect(name).To(Equal("keepalive"))
							Expect(value).To(Equal("183")) // unix timestamp

							createdContainer.Release()

							fakeClock.Increment(30 * time.Second)

							Consistently(fakeContainer.SetPropertyCallCount).Should(Equal(2))
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
						Properties: garden.Properties{
							"concourse:type":          "get",
							"concourse:pipeline-name": "some-pipeline",
							"concourse:location":      "3",
							"concourse:check-type":    "some-check-type",
							"concourse:check-source":  "{\"some\":\"source\"}",
							"concourse:name":          "some-name",
							"concourse:build-id":      "42",
						},
					}))

				})

				Context("when a root volume and inputs are provided", func() {
					var rootVolume *bfakes.FakeVolume

					var volume1 *bfakes.FakeVolume
					var volume2 *bfakes.FakeVolume

					var cowInputVolume *bfakes.FakeVolume
					var cowOtherInputVolume *bfakes.FakeVolume

					BeforeEach(func() {
						rootVolume = new(bfakes.FakeVolume)
						rootVolume.HandleReturns("root-volume")
						rootVolume.PathReturns("/some/root/src/path")

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

							if reflect.DeepEqual(spec.Strategy, baggageclaim.COWStrategy{Parent: volume1}) {
								return cowInputVolume, nil
							} else if reflect.DeepEqual(spec.Strategy, baggageclaim.COWStrategy{Parent: volume2}) {
								return cowOtherInputVolume, nil
							} else {
								Fail(fmt.Sprintf("unknown strategy: %#v", spec.Strategy))
								return nil, nil
							}
						}

						taskSpec := spec.(TaskContainerSpec)

						taskSpec.Root = VolumeMount{
							Volume:    rootVolume,
							MountPath: "/some/root/dst/path",
						}

						taskSpec.Inputs = []VolumeMount{
							{
								Volume:    volume1,
								MountPath: "/some/dst/path",
							},
							{
								Volume:    volume2,
								MountPath: "/some/other/dst/path",
							},
						}

						spec = taskSpec
					})

					It("creates the container with read-write copy-on-write bind-mounts for each input", func() {
						Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
						Expect(fakeGardenClient.CreateArgsForCall(0)).To(Equal(garden.ContainerSpec{
							RootFSPath: "some-image",
							Privileged: true,
							Properties: garden.Properties{
								"concourse:type":          "get",
								"concourse:pipeline-name": "some-pipeline",
								"concourse:location":      "3",
								"concourse:check-type":    "some-check-type",
								"concourse:check-source":  `{"some":"source"}`,
								"concourse:name":          "some-name",
								"concourse:build-id":      "42",
								"concourse:volumes":       `["root-volume","cow-input-volume","cow-other-input-volume"]`,
							},
							BindMounts: []garden.BindMount{
								{
									SrcPath: "/some/root/src/path",
									DstPath: "/some/root/dst/path",
									Mode:    garden.BindMountModeRW,
								},
								{
									SrcPath: "/some/cow/src/path",
									DstPath: "/some/dst/path",
									Mode:    garden.BindMountModeRW,
								},
								{
									SrcPath: "/some/other/cow/src/path",
									DstPath: "/some/other/dst/path",
									Mode:    garden.BindMountModeRW,
								},
							},
						}))
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
						Consistently(fakeContainer.SetPropertyCallCount).Should(BeZero())
					})

					It("is kept alive by continuously setting a keepalive property until released", func() {
						Expect(fakeContainer.SetPropertyCallCount()).To(Equal(0))
						Expect(fakeGardenWorkerDB.UpdateExpiresAtOnContainerInfoCallCount()).To(Equal(0))

						fakeClock.Increment(30 * time.Second)

						Eventually(fakeContainer.SetPropertyCallCount).Should(Equal(1))
						name, value := fakeContainer.SetPropertyArgsForCall(0)
						Expect(name).To(Equal("keepalive"))
						Expect(value).To(Equal("153")) // unix timestamp

						Eventually(fakeGardenWorkerDB.UpdateExpiresAtOnContainerInfoCallCount()).Should(Equal(1))
						handle, interval := fakeGardenWorkerDB.UpdateExpiresAtOnContainerInfoArgsForCall(0)
						Expect(handle).To(Equal("some-handle"))
						Expect(interval).To(Equal(5 * time.Minute))

						fakeClock.Increment(30 * time.Second)

						Eventually(fakeContainer.SetPropertyCallCount).Should(Equal(2))
						name, value = fakeContainer.SetPropertyArgsForCall(1)
						Expect(name).To(Equal("keepalive"))
						Expect(value).To(Equal("183")) // unix timestamp

						Eventually(fakeGardenWorkerDB.UpdateExpiresAtOnContainerInfoCallCount()).Should(Equal(2))
						handle, interval = fakeGardenWorkerDB.UpdateExpiresAtOnContainerInfoArgsForCall(1)
						Expect(handle).To(Equal("some-handle"))
						Expect(interval).To(Equal(5 * time.Minute))

						createdContainer.Release()

						fakeClock.Increment(30 * time.Second)

						Consistently(fakeContainer.SetPropertyCallCount).Should(Equal(2))
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
				foundContainer, found, err := worker.LookupContainer(logger, handle)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(foundContainer.Handle()).To(Equal(fakeContainer.Handle()))
			})

			Describe("the container", func() {
				var foundContainer Container
				var findErr error

				JustBeforeEach(func() {
					foundContainer, _, findErr = worker.LookupContainer(logger, handle)
				})

				Context("when the concourse:volumes property is present", func() {
					var handle1Volume *bfakes.FakeVolume
					var handle2Volume *bfakes.FakeVolume

					BeforeEach(func() {
						handle1Volume = new(bfakes.FakeVolume)
						handle2Volume = new(bfakes.FakeVolume)

						fakeContainer.PropertiesReturns(garden.Properties{
							"concourse:name":    name,
							"concourse:volumes": `["handle-1","handle-2"]`,
						}, nil)

						fakeBaggageclaimClient.LookupVolumeStub = func(logger lager.Logger, handle string) (baggageclaim.Volume, error) {
							if handle == "handle-1" {
								return handle1Volume, nil
							} else if handle == "handle-2" {
								return handle2Volume, nil
							} else {
								panic("unknown handle: " + handle)
							}
						}
					})

					Describe("Volumes", func() {
						It("returns all bound volumes based on properties on the container", func() {
							Expect(foundContainer.Volumes()).To(Equal([]baggageclaim.Volume{handle1Volume, handle2Volume}))
						})

						Context("when LookupVolume returns an error", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								fakeBaggageclaimClient.LookupVolumeReturns(nil, disaster)
							})

							It("returns the error on lookup", func() {
								Expect(findErr).To(Equal(disaster))
							})
						})

						Context("when there is no baggageclaim", func() {
							BeforeEach(func() {
								worker = NewGardenWorker(
									fakeGardenClient,
									nil,
									fakeGardenWorkerDB,
									fakeWorkerProvider,
									fakeClock,
									activeContainers,
									resourceTypes,
									platform,
									tags,
									name,
								)
							})

							It("returns an empty slice", func() {
								Expect(foundContainer.Volumes()).To(BeEmpty())
							})
						})
					})

					Describe("Release", func() {
						It("releases the container's volumes once and only once", func() {
							foundContainer.Release()
							Expect(handle1Volume.ReleaseCallCount()).To(Equal(1))
							Expect(handle2Volume.ReleaseCallCount()).To(Equal(1))

							foundContainer.Release()
							Expect(handle1Volume.ReleaseCallCount()).To(Equal(1))
							Expect(handle2Volume.ReleaseCallCount()).To(Equal(1))
						})
					})
				})

				Context("when the concourse:volumes property is not present", func() {
					BeforeEach(func() {
						fakeContainer.PropertiesReturns(garden.Properties{
							"concourse:name": name,
						}, nil)
					})

					Describe("Volumes", func() {
						It("returns an empty slice", func() {
							Expect(foundContainer.Volumes()).To(BeEmpty())
						})
					})
				})
			})
		})

		Context("when the gardenClient returns garden.ContaienrNotFoundError", func() {
			BeforeEach(func() {
				fakeGardenClient.LookupReturns(nil, garden.ContainerNotFoundError{"some-handle"})
			})

			It("returns false and no error", func() {
				_, found, err := worker.LookupContainer(logger, handle)
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
				foundContainer, _, err := worker.LookupContainer(logger, handle)
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
				ContainerIdentifier: db.ContainerIdentifier{
					Name: "some-name",
				},
			}
		})

		JustBeforeEach(func() {
			foundContainer, found, lookupErr = worker.FindContainerForIdentifier(logger, id)
		})

		Context("when the container can be found", func() {
			var (
				fakeContainer *gfakes.FakeContainer
				name          string
			)

			BeforeEach(func() {
				fakeContainer = new(gfakes.FakeContainer)
				fakeContainer.HandleReturns("provider-handle")

				fakeWorkerProvider.FindContainerInfoForIdentifierReturns(db.ContainerInfo{
					Handle: "provider-handle",
				}, true, nil)

				fakeGardenClient.LookupReturns(fakeContainer, nil)
			})

			It("succeeds", func() {
				Expect(lookupErr).NotTo(HaveOccurred())
			})

			It("looks for containers with matching properties via the Garden client", func() {
				Expect(fakeWorkerProvider.FindContainerInfoForIdentifierCallCount()).To(Equal(1))
				Expect(fakeWorkerProvider.FindContainerInfoForIdentifierArgsForCall(0)).To(Equal(id))

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
					Consistently(fakeContainer.SetPropertyCallCount).Should(BeZero())
				})

				It("is kept alive by continuously setting a keepalive property until released", func() {
					Expect(fakeContainer.SetPropertyCallCount()).To(Equal(0))

					fakeClock.Increment(30 * time.Second)

					Eventually(fakeContainer.SetPropertyCallCount).Should(Equal(1))
					name, value := fakeContainer.SetPropertyArgsForCall(0)
					Expect(name).To(Equal("keepalive"))
					Expect(value).To(Equal("153")) // unix timestamp

					fakeClock.Increment(30 * time.Second)

					Eventually(fakeContainer.SetPropertyCallCount).Should(Equal(2))
					name, value = fakeContainer.SetPropertyArgsForCall(1)
					Expect(name).To(Equal("keepalive"))
					Expect(value).To(Equal("183")) // unix timestamp

					foundContainer.Release()

					fakeClock.Increment(30 * time.Second)

					Consistently(fakeContainer.SetPropertyCallCount).Should(Equal(2))
				})

				It("can be released multiple times", func() {
					foundContainer.Release()
					Expect(foundContainer.Release).NotTo(Panic())
				})

				Describe("providing its Identifier", func() {
					It("can provide its Identifier", func() {
						identifier := foundContainer.IdentifierFromProperties()

						Expect(identifier.Name).To(Equal(name))
					})
				})
			})
		})

		Context("when no containers are found", func() {
			BeforeEach(func() {
				fakeWorkerProvider.FindContainerInfoForIdentifierReturns(db.ContainerInfo{}, false, nil)
			})

			It("returns that the container could not be found", func() {
				Expect(found).To(BeFalse())
			})
		})

		Context("when finding the containers fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeWorkerProvider.FindContainerInfoForIdentifierReturns(db.ContainerInfo{}, false, disaster)
			})

			It("returns the error", func() {
				Expect(lookupErr).To(Equal(disaster))
			})
		})

		Context("when looking up the container fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeWorkerProvider.FindContainerInfoForIdentifierReturns(db.ContainerInfo{Handle: "handle"}, true, nil)
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
		)

		BeforeEach(func() {
			spec = WorkerSpec{}
		})

		JustBeforeEach(func() {
			worker = NewGardenWorker(
				fakeGardenClient,
				fakeBaggageclaimClient,
				fakeGardenWorkerDB,
				fakeWorkerProvider,
				fakeClock,
				activeContainers,
				resourceTypes,
				platform,
				tags,
				name,
			)

			satisfyingWorker, satisfyingErr = worker.Satisfying(spec)
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
					Expect(satisfyingWorker).To(Equal(worker))
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
					Expect(satisfyingWorker).To(Equal(worker))
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
					Expect(satisfyingWorker).To(Equal(worker))
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
					Expect(satisfyingWorker).To(Equal(worker))
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
					Expect(satisfyingWorker).To(Equal(worker))
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
