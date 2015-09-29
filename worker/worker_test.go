package worker_test

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	gfakes "github.com/cloudfoundry-incubator/garden/fakes"
	"github.com/concourse/atc"
	. "github.com/concourse/atc/worker"
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
				Ω(volumeManager).Should(BeNil())
				Ω(hasVolumeManager).Should(BeFalse())
			})
		})

		Context("when there is a baggageclaim client", func() {
			BeforeEach(func() {
				baggageclaimClient = new(bfakes.FakeClient)
			})

			It("returns the client and true", func() {
				Ω(volumeManager).Should(Equal(baggageclaimClient))
				Ω(hasVolumeManager).Should(BeTrue())
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
				Name:         "some-name",
				PipelineName: "some-pipeline",
				BuildID:      42,
				Type:         ContainerTypeGet,
				StepLocation: 3,
				CheckType:    "some-check-type",
				CheckSource:  atc.Source{"some": "source"},
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

				Context("when creating works", func() {
					var fakeContainer *gfakes.FakeContainer

					BeforeEach(func() {
						fakeContainer = new(gfakes.FakeContainer)
						fakeContainer.HandleReturns("some-handle")

						fakeGardenClient.CreateReturns(fakeContainer, nil)
					})

					It("succeeds", func() {
						Ω(createErr).ShouldNot(HaveOccurred())
					})

					It("creates the container with the Garden client", func() {
						Ω(fakeGardenClient.CreateCallCount()).Should(Equal(1))
						Ω(fakeGardenClient.CreateArgsForCall(0)).Should(Equal(garden.ContainerSpec{
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

					Context("when env vars are provided", func() {
						BeforeEach(func() {
							spec = ResourceTypeContainerSpec{
								Type: "some-resource",
								Env:  []string{"a=1", "b=2"},
							}
						})

						It("creates the container with the given env vars", func() {
							Ω(fakeGardenClient.CreateCallCount()).Should(Equal(1))
							Ω(fakeGardenClient.CreateArgsForCall(0)).Should(Equal(garden.ContainerSpec{
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
							Ω(fakeGardenClient.CreateCallCount()).Should(Equal(1))
							Ω(fakeGardenClient.CreateArgsForCall(0)).Should(Equal(garden.ContainerSpec{
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
							Ω(fakeGardenClient.CreateCallCount()).Should(Equal(1))
							Ω(fakeGardenClient.CreateArgsForCall(0)).Should(Equal(garden.ContainerSpec{
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
							Ω(err).ShouldNot(HaveOccurred())

							By("destroying via garden")
							Ω(fakeGardenClient.DestroyCallCount()).Should(Equal(1))
							Ω(fakeGardenClient.DestroyArgsForCall(0)).Should(Equal("some-handle"))

							By("no longer heartbeating")
							fakeClock.Increment(30 * time.Second)
							Consistently(fakeContainer.SetPropertyCallCount).Should(BeZero())
						})

						It("is kept alive by continuously setting a keepalive property until released", func() {
							Ω(fakeContainer.SetPropertyCallCount()).Should(Equal(0))

							fakeClock.Increment(30 * time.Second)

							Eventually(fakeContainer.SetPropertyCallCount).Should(Equal(1))
							name, value := fakeContainer.SetPropertyArgsForCall(0)
							Ω(name).Should(Equal("keepalive"))
							Ω(value).Should(Equal("153")) // unix timestamp

							fakeClock.Increment(30 * time.Second)

							Eventually(fakeContainer.SetPropertyCallCount).Should(Equal(2))
							name, value = fakeContainer.SetPropertyArgsForCall(1)
							Ω(name).Should(Equal("keepalive"))
							Ω(value).Should(Equal("183")) // unix timestamp

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
						Ω(createErr).Should(Equal(disaster))
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
					Ω(createErr).Should(Equal(ErrUnsupportedResourceType))
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
					Ω(createErr).ShouldNot(HaveOccurred())
				})

				It("creates the container with the Garden client", func() {
					Ω(fakeGardenClient.CreateCallCount()).Should(Equal(1))
					Ω(fakeGardenClient.CreateArgsForCall(0)).Should(Equal(garden.ContainerSpec{
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
						Ω(fakeGardenClient.CreateCallCount()).Should(Equal(1))
						Ω(fakeGardenClient.CreateArgsForCall(0)).Should(Equal(garden.ContainerSpec{
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
						Ω(err).ShouldNot(HaveOccurred())

						By("destroying via garden")
						Ω(fakeGardenClient.DestroyCallCount()).Should(Equal(1))
						Ω(fakeGardenClient.DestroyArgsForCall(0)).Should(Equal("some-handle"))

						By("no longer heartbeating")
						fakeClock.Increment(30 * time.Second)
						Consistently(fakeContainer.SetPropertyCallCount).Should(BeZero())
					})

					It("is kept alive by continuously setting a keepalive property until released", func() {
						Ω(fakeContainer.SetPropertyCallCount()).Should(Equal(0))

						fakeClock.Increment(30 * time.Second)

						Eventually(fakeContainer.SetPropertyCallCount).Should(Equal(1))
						name, value := fakeContainer.SetPropertyArgsForCall(0)
						Ω(name).Should(Equal("keepalive"))
						Ω(value).Should(Equal("153")) // unix timestamp

						fakeClock.Increment(30 * time.Second)

						Eventually(fakeContainer.SetPropertyCallCount).Should(Equal(2))
						name, value = fakeContainer.SetPropertyArgsForCall(1)
						Ω(name).Should(Equal("keepalive"))
						Ω(value).Should(Equal("183")) // unix timestamp

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
					Ω(createErr).Should(Equal(disaster))
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
				foundContainer, err := worker.LookupContainer(logger, handle)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(foundContainer.Handle()).Should(Equal(fakeContainer.Handle()))
			})

			Describe("the container", func() {
				var foundContainer Container
				var findErr error

				JustBeforeEach(func() {
					foundContainer, findErr = worker.LookupContainer(logger, handle)
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
							Ω(foundContainer.Volumes()).Should(Equal([]baggageclaim.Volume{handle1Volume, handle2Volume}))
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

		Context("when the gardenClient returns an error", func() {
			var expectedErr error

			BeforeEach(func() {
				expectedErr = fmt.Errorf("container not found")
				fakeGardenClient.LookupReturns(nil, expectedErr)
			})

			It("returns nil and forwards the error", func() {
				foundContainer, err := worker.LookupContainer(logger, handle)
				Ω(err).Should(Equal(expectedErr))

				Ω(foundContainer).Should(BeNil())
			})
		})
	})

	Describe("FindContainersForIdentifiers", func() {
		var (
			id Identifier
		)

		BeforeEach(func() {
			id = Identifier{Name: "some-name"}
		})

		Context("when finding the containers succeeds", func() {
			var (
				fakeContainer *gfakes.FakeContainer
			)

			BeforeEach(func() {
				fakeContainer = new(gfakes.FakeContainer)
				fakeContainer.HandleReturns("some-handle")
				fakeContainers := []garden.Container{fakeContainer}

				fakeGardenClient.ContainersReturns(fakeContainers, nil)
			})

			It("returns the containers without error", func() {
				foundContainers, err := worker.FindContainersForIdentifier(logger, id)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(len(foundContainers)).Should(Equal(1))
				Ω(foundContainers[0].Handle()).Should(Equal(fakeContainer.Handle()))
			})
		})

		Context("when finding the containers fails", func() {
			expectedErr := errors.New("nope")

			BeforeEach(func() {
				fakeGardenClient.ContainersReturns(nil, expectedErr)
			})

			It("returns nil and forwards the error", func() {
				foundContainers, err := worker.FindContainersForIdentifier(logger, id)

				Ω(err).Should(Equal(expectedErr))
				Ω(foundContainers).Should(BeNil())
			})
		})
	})

	Describe("FindContainerForIdentifier", func() {
		var (
			id Identifier

			foundContainer Container
			lookupErr      error
		)

		BeforeEach(func() {
			id = Identifier{Name: "some-name"}
		})

		JustBeforeEach(func() {
			foundContainer, lookupErr = worker.FindContainerForIdentifier(logger, id)
		})

		Context("when the container can be found", func() {
			var (
				fakeContainer *gfakes.FakeContainer
				name          string
			)

			BeforeEach(func() {
				fakeContainer = new(gfakes.FakeContainer)
				fakeContainer.HandleReturns("some-handle")

				fakeGardenClient.ContainersReturns([]garden.Container{fakeContainer}, nil)

				name = "some-name"
				fakeContainer.PropertiesReturns(garden.Properties{
					"concourse:name": name,
				}, nil)
			})

			It("succeeds", func() {
				Ω(lookupErr).ShouldNot(HaveOccurred())
			})

			It("looks for containers with matching properties via the Garden client", func() {
				Ω(fakeGardenClient.ContainersCallCount()).Should(Equal(1))
				Ω(fakeGardenClient.ContainersArgsForCall(0)).Should(Equal(garden.Properties{
					"concourse:name": name,
				}))
			})

			Describe("the found container", func() {
				It("can be destroyed", func() {
					err := foundContainer.Destroy()
					Ω(err).ShouldNot(HaveOccurred())

					By("destroying via garden")
					Ω(fakeGardenClient.DestroyCallCount()).Should(Equal(1))
					Ω(fakeGardenClient.DestroyArgsForCall(0)).Should(Equal("some-handle"))

					By("no longer heartbeating")
					fakeClock.Increment(30 * time.Second)
					Consistently(fakeContainer.SetPropertyCallCount).Should(BeZero())
				})

				It("is kept alive by continuously setting a keepalive property until released", func() {
					Ω(fakeContainer.SetPropertyCallCount()).Should(Equal(0))

					fakeClock.Increment(30 * time.Second)

					Eventually(fakeContainer.SetPropertyCallCount).Should(Equal(1))
					name, value := fakeContainer.SetPropertyArgsForCall(0)
					Ω(name).Should(Equal("keepalive"))
					Ω(value).Should(Equal("153")) // unix timestamp

					fakeClock.Increment(30 * time.Second)

					Eventually(fakeContainer.SetPropertyCallCount).Should(Equal(2))
					name, value = fakeContainer.SetPropertyArgsForCall(1)
					Ω(name).Should(Equal("keepalive"))
					Ω(value).Should(Equal("183")) // unix timestamp

					foundContainer.Release()

					fakeClock.Increment(30 * time.Second)

					Consistently(fakeContainer.SetPropertyCallCount).Should(Equal(2))
				})

				It("can be released multiple times", func() {
					foundContainer.Release()
					Ω(foundContainer.Release).ShouldNot(Panic())
				})

				Describe("providing its Identifier", func() {
					It("can provide its Identifier", func() {
						identifier := foundContainer.IdentifierFromProperties()

						Ω(identifier.Name).Should(Equal(name))
					})
				})
			})
		})

		Context("when multiple containers are found", func() {
			var fakeContainer *gfakes.FakeContainer
			var bonusContainer *gfakes.FakeContainer

			BeforeEach(func() {
				fakeContainer = new(gfakes.FakeContainer)
				fakeContainer.HandleReturns("some-handle")

				bonusContainer = new(gfakes.FakeContainer)
				bonusContainer.HandleReturns("some-other-handle")

				fakeGardenClient.ContainersReturns([]garden.Container{fakeContainer, bonusContainer}, nil)
			})

			It("returns ErrMultipleContainers", func() {
				Ω(lookupErr).Should(Equal(MultipleContainersError{
					Handles: []string{"some-handle", "some-other-handle"},
				}))
			})
		})

		Context("when no containers are found", func() {
			BeforeEach(func() {
				fakeGardenClient.ContainersReturns([]garden.Container{}, nil)
			})

			It("returns ErrContainerNotFound", func() {
				Ω(lookupErr).Should(Equal(ErrContainerNotFound))
			})
		})

		Context("when finding the containers fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeGardenClient.ContainersReturns(nil, disaster)
			})

			It("returns the error", func() {
				Ω(lookupErr).Should(Equal(disaster))
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
					Ω(satisfyingErr).Should(Equal(ErrMismatchedTags))
				})
			})

			Context("when the worker has no tags", func() {
				BeforeEach(func() {
					tags = []string{}
				})

				It("returns the worker", func() {
					Ω(satisfyingWorker).Should(Equal(worker))
				})

				It("returns no error", func() {
					Ω(satisfyingErr).ShouldNot(HaveOccurred())
				})
			})

			Context("when all of the requested tags are present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"some", "tags"}
				})

				It("returns the worker", func() {
					Ω(satisfyingWorker).Should(Equal(worker))
				})

				It("returns no error", func() {
					Ω(satisfyingErr).ShouldNot(HaveOccurred())
				})
			})

			Context("when some of the requested tags are present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"some"}
				})

				It("returns the worker", func() {
					Ω(satisfyingWorker).Should(Equal(worker))
				})

				It("returns no error", func() {
					Ω(satisfyingErr).ShouldNot(HaveOccurred())
				})
			})

			Context("when any of the requested tags are not present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"bogus", "tags"}
				})

				It("returns ErrMismatchedTags", func() {
					Ω(satisfyingErr).Should(Equal(ErrMismatchedTags))
				})
			})
		})

		Context("when the platform is incompatible", func() {
			BeforeEach(func() {
				spec.Platform = "some-bogus-platform"
			})

			It("returns ErrIncompatiblePlatform", func() {
				Ω(satisfyingErr).Should(Equal(ErrIncompatiblePlatform))
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
					Ω(satisfyingWorker).Should(Equal(worker))
				})

				It("returns no error", func() {
					Ω(satisfyingErr).ShouldNot(HaveOccurred())
				})
			})

			Context("when some of the requested tags are present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"some"}
				})

				It("returns the worker", func() {
					Ω(satisfyingWorker).Should(Equal(worker))
				})

				It("returns no error", func() {
					Ω(satisfyingErr).ShouldNot(HaveOccurred())
				})
			})

			Context("when any of the requested tags are not present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"bogus", "tags"}
				})

				It("returns ErrMismatchedTags", func() {
					Ω(satisfyingErr).Should(Equal(ErrMismatchedTags))
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
					Ω(satisfyingErr).Should(Equal(ErrUnsupportedResourceType))
				})
			})

			Context("when some of the requested tags are present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"some"}
				})

				It("returns ErrUnsupportedResourceType", func() {
					Ω(satisfyingErr).Should(Equal(ErrUnsupportedResourceType))
				})
			})

			Context("when any of the requested tags are not present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"bogus", "tags"}
				})

				It("returns ErrUnsupportedResourceType", func() {
					Ω(satisfyingErr).Should(Equal(ErrUnsupportedResourceType))
				})
			})
		})
	})

	Describe("Name", func() {
		It("responds correctly", func() {
			Ω(worker.Name()).To(Equal(name))
		})
	})
})
