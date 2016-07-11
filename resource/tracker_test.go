package resource_test

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource/resourcefakes"
	"github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/workerfakes"
	"github.com/concourse/baggageclaim"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/concourse/atc/resource"
)

type testMetadata []string

func (m testMetadata) Env() []string { return m }

var _ = Describe("Tracker", func() {
	var (
		tracker     Tracker
		customTypes atc.ResourceTypes
	)

	var session = Session{
		ID: worker.Identifier{},
		Metadata: worker.Metadata{
			WorkerName:           "some-worker",
			EnvironmentVariables: []string{"some=value"},
		},
		Ephemeral: true,
	}

	BeforeEach(func() {
		tracker = NewTracker(workerClient)
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

	Describe("Init", func() {
		var (
			logger   *lagertest.TestLogger
			metadata Metadata = testMetadata{"a=1", "b=2"}
			delegate worker.ImageFetchingDelegate

			initType ResourceType

			initResource Resource
			initErr      error
		)

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("test")
			initType = "type1"
			delegate = new(wfakes.FakeImageFetchingDelegate)

			workerClient.CreateContainerReturns(fakeContainer, nil)
		})

		JustBeforeEach(func() {
			initResource, initErr = tracker.Init(logger, metadata, session, initType, []string{"resource", "tags"}, customTypes, delegate)
		})

		Context("when a container does not exist for the session", func() {
			BeforeEach(func() {
				workerClient.FindContainerForIdentifierReturns(nil, false, nil)
			})

			It("does not error and returns a resource", func() {
				Expect(initErr).NotTo(HaveOccurred())
				Expect(initResource).NotTo(BeNil())
			})

			It("creates a container with the resource's type, env, ephemeral information, and the session as the handle", func() {
				_, _, _, id, containerMetadata, spec, actualCustomTypes := workerClient.CreateContainerArgsForCall(0)

				Expect(id).To(Equal(session.ID))
				Expect(containerMetadata).To(Equal(session.Metadata))

				Expect(spec.Platform).To(BeEmpty())
				Expect(spec.Tags).To(ConsistOf("resource", "tags"))
				Expect(spec.ImageSpec).To(Equal(worker.ImageSpec{
					ResourceType: string(initType),
					Privileged:   true,
				}))
				Expect(spec.Ephemeral).To(BeTrue())
				Expect(spec.Env).To(Equal([]string{"a=1", "b=2"}))
				Expect(spec.Inputs).To(BeEmpty())
				Expect(spec.Outputs).To(BeEmpty())

				Expect(actualCustomTypes).To(Equal(customTypes))
			})

			Context("when creating the container fails", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					workerClient.CreateContainerReturns(nil, disaster)
				})

				It("returns the error and no resource", func() {
					Expect(initErr).To(Equal(disaster))
					Expect(initResource).To(BeNil())
				})
			})
		})

		Context("when looking up the container fails for some reason", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				workerClient.FindContainerForIdentifierReturns(nil, false, disaster)
			})

			It("returns the error and no resource", func() {
				Expect(initErr).To(Equal(disaster))
				Expect(initResource).To(BeNil())
			})

			It("does not create a container", func() {
				Expect(workerClient.CreateContainerCallCount()).To(BeZero())
			})
		})

		Context("when a container already exists for the session", func() {
			var fakeContainer *wfakes.FakeContainer

			BeforeEach(func() {
				fakeContainer = new(wfakes.FakeContainer)
				workerClient.FindContainerForIdentifierReturns(fakeContainer, true, nil)
			})

			It("does not error and returns a resource", func() {
				Expect(initErr).NotTo(HaveOccurred())
				Expect(initResource).NotTo(BeNil())
			})

			It("does not create a container", func() {
				Expect(workerClient.CreateContainerCallCount()).To(BeZero())
			})
		})
	})

	Describe("ChooseWorker", func() {
		It("chose the worker satisfying the resource type and tags", func() {
			tracker.ChooseWorker("fake-resource-type", atc.Tags{"resource", "tags"}, customTypes)

			actualSpec, actualCustomTypes := workerClient.SatisfyingArgsForCall(0)
			Expect(actualSpec).To(Equal(
				worker.WorkerSpec{
					ResourceType: "fake-resource-type",
					Tags:         []string{"resource", "tags"},
				},
			))
			Expect(actualCustomTypes).To(Equal(customTypes))
		})

		Context("when a worker is found", func() {
			var satisfyingWorker *wfakes.FakeWorker

			BeforeEach(func() {
				satisfyingWorker = new(wfakes.FakeWorker)
				workerClient.SatisfyingReturns(satisfyingWorker, nil)

				satisfyingWorker.CreateContainerReturns(fakeContainer, nil)
			})

			It("returns found worker", func() {
				foundWorker, err := tracker.ChooseWorker("fake-resource-type",
					atc.Tags{"tag-1", "tag-2"},
					atc.ResourceTypes{
						{Name: "fake-resource-type"},
					})

				Expect(err).NotTo(HaveOccurred())
				Expect(foundWorker).To(Equal(satisfyingWorker))
			})
		})

		Context("when no worker satisfies the spec", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				workerClient.SatisfyingReturns(nil, disaster)
			})

			It("returns the error and no resource", func() {
				foundWorker, err := tracker.ChooseWorker("fake-resource-type",
					atc.Tags{"tag-1", "tag-2"},
					atc.ResourceTypes{
						{Name: "fake-resource-type"},
					})

				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(disaster))
				Expect(foundWorker).To(BeNil())
			})
		})
	})

	Describe("FindContainerForSession", func() {
		var logger *lagertest.TestLogger
		var foundResource Resource
		var foundCache Cache
		var found bool
		var findErr error

		JustBeforeEach(func() {
			logger = lagertest.NewTestLogger("test")
			foundResource, foundCache, found, findErr = tracker.FindContainerForSession(logger, session)
		})

		Context("when a container does not exist for the session", func() {
			BeforeEach(func() {
				workerClient.FindContainerForIdentifierReturns(nil, false, nil)
			})

			It("returns false", func() {
				Expect(findErr).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when looking up the container fails for some reason", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				workerClient.FindContainerForIdentifierReturns(nil, false, disaster)
			})

			It("returns the error and no resource", func() {
				Expect(findErr).To(HaveOccurred())
				Expect(findErr).To(Equal(disaster))
			})
		})

		Context("when a container already exists for the session", func() {
			var fakeContainer *wfakes.FakeContainer

			BeforeEach(func() {
				fakeContainer = new(wfakes.FakeContainer)
				workerClient.FindContainerForIdentifierReturns(fakeContainer, true, nil)
			})

			It("does not error and returns a resource", func() {
				Expect(findErr).NotTo(HaveOccurred())
				Expect(foundResource).NotTo(BeNil())
			})

			Context("when the container has a cache volume", func() {
				var cacheVolume *wfakes.FakeVolume
				var otherVolume *wfakes.FakeVolume

				BeforeEach(func() {
					cacheVolume = new(wfakes.FakeVolume)
					otherVolume = new(wfakes.FakeVolume)

					fakeContainer.VolumeMountsReturns([]worker.VolumeMount{
						{
							Volume:    otherVolume,
							MountPath: "/tmp/build/forgetaboutit",
						},
						{
							Volume:    cacheVolume,
							MountPath: "/tmp/build/get",
						},
					})
				})

				Describe("the cache", func() {
					Describe("IsInitialized", func() {
						Context("when the volume has the initialized property set", func() {
							BeforeEach(func() {
								cacheVolume.PropertiesReturns(baggageclaim.VolumeProperties{
									"initialized": "any-value",
								}, nil)
							})

							It("returns true", func() {
								Expect(foundCache.IsInitialized()).To(BeTrue())
							})
						})

						Context("when the volume has no initialized property", func() {
							BeforeEach(func() {
								cacheVolume.PropertiesReturns(baggageclaim.VolumeProperties{}, nil)
							})

							It("returns false", func() {
								initialized, err := foundCache.IsInitialized()
								Expect(initialized).To(BeFalse())
								Expect(err).ToNot(HaveOccurred())
							})
						})

						Context("when getting the properties fails", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								cacheVolume.PropertiesReturns(nil, disaster)
							})

							It("returns the error", func() {
								_, err := foundCache.IsInitialized()
								Expect(err).To(Equal(disaster))
							})
						})
					})

					Describe("Initialize", func() {
						It("sets the initialized property on the volume", func() {
							Expect(foundCache.Initialize()).To(Succeed())

							Expect(cacheVolume.SetPropertyCallCount()).To(Equal(1))
							name, value := cacheVolume.SetPropertyArgsForCall(0)
							Expect(name).To(Equal("initialized"))
							Expect(value).To(Equal("yep"))
						})

						Context("when setting the property fails", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								cacheVolume.SetPropertyReturns(disaster)
							})

							It("returns the error", func() {
								err := foundCache.Initialize()
								Expect(err).To(Equal(disaster))
							})
						})
					})
				})
			})

			Context("when the container has volumes but none of them are the cache", func() {
				var otherVolume *wfakes.FakeVolume

				BeforeEach(func() {
					otherVolume = new(wfakes.FakeVolume)

					fakeContainer.VolumeMountsReturns([]worker.VolumeMount{
						{
							Volume:    otherVolume,
							MountPath: "/tmp/build/forgetaboutit",
						},
					})
				})

				Describe("the cache", func() {
					It("is not initialized", func() {
						initialized, err := foundCache.IsInitialized()
						Expect(initialized).To(BeFalse())
						Expect(err).ToNot(HaveOccurred())
					})

					It("does a no-op initialize", func() {
						Expect(foundCache.Initialize()).To(Succeed())
					})
				})
			})

			Context("when the container has no volumes", func() {
				BeforeEach(func() {
					fakeContainer.VolumeMountsReturns([]worker.VolumeMount{})
				})

				Describe("the cache", func() {
					It("is not initialized", func() {
						initialized, err := foundCache.IsInitialized()
						Expect(initialized).To(BeFalse())
						Expect(err).ToNot(HaveOccurred())
					})

					It("does a no-op initialize", func() {
						Expect(foundCache.Initialize()).To(Succeed())
					})
				})
			})
		})
	})

	Describe("InitWithCache", func() {
		var (
			logger        *lagertest.TestLogger
			metadata      Metadata = testMetadata{"a=1", "b=2"}
			delegate      worker.ImageFetchingDelegate
			choosenWorker *wfakes.FakeWorker

			initType        ResourceType
			cacheIdentifier *resourcefakes.FakeCacheIdentifier

			initResource Resource
			initCache    Cache
			initErr      error
		)

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("test")
			initType = "type1"
			cacheIdentifier = new(resourcefakes.FakeCacheIdentifier)
			delegate = new(wfakes.FakeImageFetchingDelegate)
			choosenWorker = new(wfakes.FakeWorker)
		})

		JustBeforeEach(func() {
			initResource, initCache, initErr = tracker.InitWithCache(
				logger,
				metadata,
				session,
				initType,
				[]string{"resource", "tags"},
				cacheIdentifier,
				customTypes,
				delegate,
				choosenWorker,
			)
		})

		BeforeEach(func() {
			choosenWorker.CreateContainerReturns(fakeContainer, nil)
		})

		Context("when the cache is already present", func() {
			var foundVolume *wfakes.FakeVolume

			BeforeEach(func() {
				foundVolume = new(wfakes.FakeVolume)
				foundVolume.HandleReturns("found-volume-handle")
				cacheIdentifier.FindOnReturns(foundVolume, true, nil)

				cacheIdentifier.VolumeIdentifierReturns(worker.VolumeIdentifier{
					ResourceCache: &db.ResourceCacheIdentifier{
						ResourceVersion: atc.Version{"some": "theversion"},
						ResourceHash:    "hash",
					},
				})

				choosenWorker.NameReturns("some-worker")
			})

			It("does not error and returns a resource", func() {
				Expect(initErr).NotTo(HaveOccurred())
				Expect(initResource).NotTo(BeNil())
			})

			It("located it on the correct worker", func() {
				Expect(cacheIdentifier.FindOnCallCount()).To(Equal(1))
				_, workerClient := cacheIdentifier.FindOnArgsForCall(0)
				Expect(workerClient).To(Equal(choosenWorker))
			})

			It("creates the container with the cache volume", func() {
				_, _, _, id, containerMetadata, spec, actualCustomTypes := choosenWorker.CreateContainerArgsForCall(0)

				Expect(id).To(Equal(session.ID))
				Expect(containerMetadata).To(Equal(session.Metadata))

				Expect(spec.Platform).To(BeEmpty())
				Expect(spec.Tags).To(ConsistOf("resource", "tags"))
				Expect(spec.ImageSpec).To(Equal(worker.ImageSpec{
					ResourceType: string(initType),
					Privileged:   true,
				}))
				Expect(spec.Ephemeral).To(BeTrue())
				Expect(spec.Env).To(Equal([]string{"a=1", "b=2"}))
				Expect(spec.Inputs).To(BeEmpty())
				Expect(spec.Outputs).To(ConsistOf(worker.VolumeMount{
					Volume:    foundVolume,
					MountPath: "/tmp/build/get",
				}))

				Expect(actualCustomTypes).To(Equal(customTypes))
			})

			It("releases the volume, since the container keeps it alive", func() {
				Expect(foundVolume.ReleaseCallCount()).To(Equal(1))
			})

			Describe("the cache", func() {
				Describe("IsInitialized", func() {
					Context("when the volume has the initialized property set", func() {
						BeforeEach(func() {
							foundVolume.PropertiesReturns(baggageclaim.VolumeProperties{
								"initialized": "any-value",
							}, nil)
						})

						It("returns true", func() {
							Expect(initCache.IsInitialized()).To(BeTrue())
						})
					})

					Context("when the volume has no initialized property", func() {
						BeforeEach(func() {
							foundVolume.PropertiesReturns(baggageclaim.VolumeProperties{}, nil)
						})

						It("returns false", func() {
							initialized, err := initCache.IsInitialized()
							Expect(initialized).To(BeFalse())
							Expect(err).ToNot(HaveOccurred())
						})
					})

					Context("when getting the properties fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							foundVolume.PropertiesReturns(nil, disaster)
						})

						It("returns the error", func() {
							_, err := initCache.IsInitialized()
							Expect(err).To(Equal(disaster))
						})
					})
				})

				Describe("Initialize", func() {
					It("sets the initialized property on the volume", func() {
						Expect(initCache.Initialize()).To(Succeed())

						Expect(foundVolume.SetPropertyCallCount()).To(Equal(1))
						name, value := foundVolume.SetPropertyArgsForCall(0)
						Expect(name).To(Equal("initialized"))
						Expect(value).To(Equal("yep"))
					})

					Context("when setting the property fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							foundVolume.SetPropertyReturns(disaster)
						})

						It("returns the error", func() {
							err := initCache.Initialize()
							Expect(err).To(Equal(disaster))
						})
					})
				})
			})
		})

		Context("when an initialized volume for the cache is not present", func() {
			BeforeEach(func() {
				cacheIdentifier.FindOnReturns(nil, false, nil)
			})

			Context("when creating the cache succeeds", func() {
				var createdVolume *wfakes.FakeVolume

				BeforeEach(func() {
					createdVolume = new(wfakes.FakeVolume)
					createdVolume.HandleReturns("created-volume-handle")

					cacheIdentifier.CreateOnReturns(createdVolume, nil)
				})

				It("does not error and returns a resource", func() {
					Expect(initErr).NotTo(HaveOccurred())
					Expect(initResource).NotTo(BeNil())
				})

				It("created the volume on the right worker", func() {
					Expect(cacheIdentifier.CreateOnCallCount()).To(Equal(1))
					_, workerClient := cacheIdentifier.CreateOnArgsForCall(0)
					Expect(workerClient).To(Equal(choosenWorker))
				})

				It("creates the container with the created cache volume", func() {
					_, _, _, id, containerMetadata, spec, actualCustomTypes := choosenWorker.CreateContainerArgsForCall(0)

					Expect(id).To(Equal(session.ID))
					Expect(containerMetadata).To(Equal(session.Metadata))

					Expect(spec.Platform).To(BeEmpty())
					Expect(spec.Tags).To(ConsistOf("resource", "tags"))
					Expect(spec.ImageSpec).To(Equal(worker.ImageSpec{
						ResourceType: string(initType),
						Privileged:   true,
					}))
					Expect(spec.Ephemeral).To(BeTrue())
					Expect(spec.Env).To(Equal([]string{"a=1", "b=2"}))
					Expect(spec.Inputs).To(BeEmpty())
					Expect(spec.Outputs).To(ConsistOf(worker.VolumeMount{
						Volume:    createdVolume,
						MountPath: "/tmp/build/get",
					}))

					Expect(actualCustomTypes).To(Equal(customTypes))
				})

				It("releases the volume, since the container keeps it alive", func() {
					Expect(createdVolume.ReleaseCallCount()).To(Equal(1))
				})

				Describe("the cache", func() {
					Describe("IsInitialized", func() {
						Context("when the volume has the initialized property set", func() {
							BeforeEach(func() {
								createdVolume.PropertiesReturns(baggageclaim.VolumeProperties{
									"initialized": "any-value",
								}, nil)
							})

							It("returns true", func() {
								Expect(initCache.IsInitialized()).To(BeTrue())
							})
						})

						Context("when the volume has no initialized property", func() {
							BeforeEach(func() {
								createdVolume.PropertiesReturns(baggageclaim.VolumeProperties{}, nil)
							})

							It("returns false", func() {
								initialized, err := initCache.IsInitialized()
								Expect(initialized).To(BeFalse())
								Expect(err).ToNot(HaveOccurred())
							})
						})

						Context("when getting the properties fails", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								createdVolume.PropertiesReturns(nil, disaster)
							})

							It("returns the error", func() {
								_, err := initCache.IsInitialized()
								Expect(err).To(Equal(disaster))
							})
						})
					})

					Describe("Initialize", func() {
						It("sets the initialized property on the volume", func() {
							Expect(initCache.Initialize()).To(Succeed())

							Expect(createdVolume.SetPropertyCallCount()).To(Equal(1))
							name, value := createdVolume.SetPropertyArgsForCall(0)
							Expect(name).To(Equal("initialized"))
							Expect(value).To(Equal("yep"))
						})

						Context("when setting the property fails", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								createdVolume.SetPropertyReturns(disaster)
							})

							It("returns the error", func() {
								err := initCache.Initialize()
								Expect(err).To(Equal(disaster))
							})
						})
					})
				})
			})
		})
	})

	Describe("InitWithSources", func() {
		var (
			logger       *lagertest.TestLogger
			metadata     Metadata = testMetadata{"a=1", "b=2"}
			inputSources map[string]ArtifactSource
			delegate     worker.ImageFetchingDelegate

			inputSource1 *resourcefakes.FakeArtifactSource
			inputSource2 *resourcefakes.FakeArtifactSource
			inputSource3 *resourcefakes.FakeArtifactSource

			initType ResourceType

			initResource   Resource
			missingSources []string
			initErr        error
		)

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("test")
			initType = "type1"
			delegate = new(wfakes.FakeImageFetchingDelegate)

			inputSource1 = new(resourcefakes.FakeArtifactSource)
			inputSource2 = new(resourcefakes.FakeArtifactSource)
			inputSource3 = new(resourcefakes.FakeArtifactSource)

			inputSources = map[string]ArtifactSource{
				"source-1-name": inputSource1,
				"source-2-name": inputSource2,
				"source-3-name": inputSource3,
			}
		})

		JustBeforeEach(func() {
			initResource, missingSources, initErr = tracker.InitWithSources(
				logger,
				metadata,
				session,
				initType,
				[]string{"resource", "tags"},
				inputSources,
				customTypes,
				delegate,
			)
		})

		Context("when a container does not exist for the session", func() {
			BeforeEach(func() {
				workerClient.FindContainerForIdentifierReturns(nil, false, nil)
			})

			Context("when a worker is found", func() {
				var satisfyingWorker *wfakes.FakeWorker

				BeforeEach(func() {
					satisfyingWorker = new(wfakes.FakeWorker)
					workerClient.AllSatisfyingReturns([]worker.Worker{satisfyingWorker}, nil)

					satisfyingWorker.CreateContainerReturns(fakeContainer, nil)
				})

				Context("when some volumes are found on the worker", func() {
					var (
						inputVolume1 *wfakes.FakeVolume
						inputVolume3 *wfakes.FakeVolume
					)

					BeforeEach(func() {
						inputVolume1 = new(wfakes.FakeVolume)
						inputVolume3 = new(wfakes.FakeVolume)

						inputSource1.VolumeOnReturns(inputVolume1, true, nil)
						inputSource2.VolumeOnReturns(nil, false, nil)
						inputSource3.VolumeOnReturns(inputVolume3, true, nil)
					})

					It("does not error and returns a resource", func() {
						Expect(initErr).NotTo(HaveOccurred())
						Expect(initResource).NotTo(BeNil())
					})

					It("chose the worker satisfying the resource type and tags", func() {
						Expect(workerClient.AllSatisfyingCallCount()).To(Equal(1))
						actualSpec, actualCustomTypes := workerClient.AllSatisfyingArgsForCall(0)
						Expect(actualSpec).To(Equal(
							worker.WorkerSpec{
								ResourceType: "type1",
								Tags:         []string{"resource", "tags"},
							},
						))
						Expect(actualCustomTypes).To(Equal(customTypes))
					})

					It("looked for the sources on the correct worker", func() {
						Expect(inputSource1.VolumeOnCallCount()).To(Equal(1))
						actualWorker := inputSource1.VolumeOnArgsForCall(0)
						Expect(actualWorker).To(Equal(satisfyingWorker))

						Expect(inputSource2.VolumeOnCallCount()).To(Equal(1))
						actualWorker = inputSource2.VolumeOnArgsForCall(0)
						Expect(actualWorker).To(Equal(satisfyingWorker))

						Expect(inputSource3.VolumeOnCallCount()).To(Equal(1))
						actualWorker = inputSource3.VolumeOnArgsForCall(0)
						Expect(actualWorker).To(Equal(satisfyingWorker))
					})

					It("creates the container with the cache volume", func() {
						Expect(satisfyingWorker.CreateContainerCallCount()).To(Equal(1))
						_, _, _, id, containerMetadata, spec, actualCustomTypes := satisfyingWorker.CreateContainerArgsForCall(0)

						Expect(id).To(Equal(session.ID))
						Expect(containerMetadata).To(Equal(session.Metadata))

						Expect(spec.Platform).To(BeEmpty())
						Expect(spec.Tags).To(ConsistOf("resource", "tags"))
						Expect(spec.ImageSpec).To(Equal(worker.ImageSpec{
							ResourceType: string(initType),
							Privileged:   true,
						}))
						Expect(spec.Ephemeral).To(BeTrue())
						Expect(spec.Env).To(Equal([]string{"a=1", "b=2"}))
						Expect(spec.Inputs).To(ConsistOf([]worker.VolumeMount{
							{
								Volume:    inputVolume1,
								MountPath: "/tmp/build/put/source-1-name",
							},
							{
								Volume:    inputVolume3,
								MountPath: "/tmp/build/put/source-3-name",
							},
						}))
						Expect(spec.Outputs).To(BeEmpty())

						Expect(actualCustomTypes).To(Equal(customTypes))
					})

					It("releases the volume, since the container keeps it alive", func() {
						Expect(inputVolume1.ReleaseCallCount()).To(Equal(1))
						Expect(inputVolume3.ReleaseCallCount()).To(Equal(1))
					})

					It("returns the artifact sources that it could not find volumes for", func() {
						Expect(missingSources).To(ConsistOf("source-2-name"))
					})

					Context("when creating the container fails", func() {
						disaster := errors.New("oh no!")

						BeforeEach(func() {
							satisfyingWorker.CreateContainerReturns(nil, disaster)
						})

						It("returns the error and no resource", func() {
							Expect(initErr).To(Equal(disaster))
							Expect(missingSources).To(BeNil())
							Expect(initResource).To(BeNil())
						})
					})
				})

				Context("when there are no volumes on the container (e.g. doesn't support volumes)", func() {
					BeforeEach(func() {
						inputSource1.VolumeOnReturns(nil, false, nil)
						inputSource2.VolumeOnReturns(nil, false, nil)
						inputSource3.VolumeOnReturns(nil, false, nil)
					})

					It("creates a container with no volumes", func() {
						Expect(satisfyingWorker.CreateContainerCallCount()).To(Equal(1))
						_, _, _, id, containerMetadata, spec, actualCustomTypes := satisfyingWorker.CreateContainerArgsForCall(0)

						Expect(id).To(Equal(session.ID))
						Expect(containerMetadata).To(Equal(session.Metadata))

						Expect(spec.Platform).To(BeEmpty())
						Expect(spec.Tags).To(ConsistOf("resource", "tags"))
						Expect(spec.ImageSpec).To(Equal(worker.ImageSpec{
							ResourceType: string(initType),
							Privileged:   true,
						}))
						Expect(spec.Ephemeral).To(BeTrue())
						Expect(spec.Env).To(Equal([]string{"a=1", "b=2"}))
						Expect(spec.Inputs).To(BeEmpty())
						Expect(spec.Outputs).To(BeEmpty())

						Expect(actualCustomTypes).To(Equal(customTypes))
					})

					It("returns them all as missing sources", func() {
						Expect(missingSources).To(ConsistOf("source-1-name", "source-2-name", "source-3-name"))
					})
				})

				Context("when looking up one of the volumes fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						inputSource1.VolumeOnReturns(nil, false, nil)
						inputSource2.VolumeOnReturns(nil, false, disaster)
						inputSource3.VolumeOnReturns(nil, false, nil)
					})

					It("returns the error and no resource", func() {
						Expect(initErr).To(Equal(disaster))
						Expect(missingSources).To(BeNil())
						Expect(initResource).To(BeNil())
					})
				})
			})

			Context("when multiple workers satisfy the spec", func() {
				var (
					satisfyingWorker1 *wfakes.FakeWorker
					satisfyingWorker2 *wfakes.FakeWorker
					satisfyingWorker3 *wfakes.FakeWorker
				)

				BeforeEach(func() {
					satisfyingWorker1 = new(wfakes.FakeWorker)
					satisfyingWorker2 = new(wfakes.FakeWorker)
					satisfyingWorker3 = new(wfakes.FakeWorker)

					workerClient.AllSatisfyingReturns([]worker.Worker{
						satisfyingWorker1,
						satisfyingWorker2,
						satisfyingWorker3,
					}, nil)

					satisfyingWorker1.CreateContainerReturns(fakeContainer, nil)
					satisfyingWorker2.CreateContainerReturns(fakeContainer, nil)
					satisfyingWorker3.CreateContainerReturns(fakeContainer, nil)
				})

				Context("and some workers have more matching input volumes than others", func() {
					var inputVolume *wfakes.FakeVolume
					var inputVolume2 *wfakes.FakeVolume
					var inputVolume3 *wfakes.FakeVolume
					var otherInputVolume *wfakes.FakeVolume

					BeforeEach(func() {
						inputVolume = new(wfakes.FakeVolume)
						inputVolume.HandleReturns("input-volume-1")

						inputVolume2 = new(wfakes.FakeVolume)
						inputVolume2.HandleReturns("input-volume-2")

						inputVolume3 = new(wfakes.FakeVolume)
						inputVolume3.HandleReturns("input-volume-3")

						otherInputVolume = new(wfakes.FakeVolume)
						otherInputVolume.HandleReturns("other-input-volume")

						inputSource1.VolumeOnStub = func(w worker.Worker) (worker.Volume, bool, error) {
							if w == satisfyingWorker1 {
								return inputVolume, true, nil
							} else if w == satisfyingWorker2 {
								return inputVolume2, true, nil
							} else if w == satisfyingWorker3 {
								return inputVolume3, true, nil
							} else {
								return nil, false, fmt.Errorf("unexpected worker: %#v\n", w)
							}
						}

						inputSource2.VolumeOnStub = func(w worker.Worker) (worker.Volume, bool, error) {
							if w == satisfyingWorker1 {
								return nil, false, nil
							} else if w == satisfyingWorker2 {
								return otherInputVolume, true, nil
							} else if w == satisfyingWorker3 {
								return nil, false, nil
							} else {
								return nil, false, fmt.Errorf("unexpected worker: %#v\n", w)
							}
						}
						inputSource3.VolumeOnReturns(nil, false, nil)

						satisfyingWorker1.CreateContainerReturns(nil, errors.New("fall out of method here"))
						satisfyingWorker2.CreateContainerReturns(nil, errors.New("fall out of method here"))
						satisfyingWorker3.CreateContainerReturns(nil, errors.New("fall out of method here"))
					})

					It("picks the worker that has the most", func() {
						Expect(satisfyingWorker1.CreateContainerCallCount()).To(Equal(0))
						Expect(satisfyingWorker2.CreateContainerCallCount()).To(Equal(1))
						Expect(satisfyingWorker3.CreateContainerCallCount()).To(Equal(0))
					})

					It("releases the volumes on the unused workers", func() {
						Expect(inputVolume.ReleaseCallCount()).To(Equal(1))
						Expect(inputVolume3.ReleaseCallCount()).To(Equal(1))

						// We don't expect these to be released because we are
						// causing an error in the create container step, which
						// happens before they are released.
						Expect(inputVolume2.ReleaseCallCount()).To(Equal(0))
						Expect(otherInputVolume.ReleaseCallCount()).To(Equal(0))
					})
				})
			})

			Context("when no worker satisfies the spec", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					workerClient.AllSatisfyingReturns(nil, disaster)
				})

				It("returns the error and no resource", func() {
					Expect(initErr).To(Equal(disaster))
					Expect(missingSources).To(BeNil())
					Expect(initResource).To(BeNil())
				})
			})
		})

		Context("when looking up the container fails for some reason", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				workerClient.FindContainerForIdentifierReturns(nil, false, disaster)
			})

			It("returns the error and no resource", func() {
				Expect(initErr).To(Equal(disaster))
				Expect(missingSources).To(BeNil())
				Expect(initResource).To(BeNil())
			})

			It("does not create a container", func() {
				Expect(workerClient.SatisfyingCallCount()).To(BeZero())
				Expect(workerClient.CreateContainerCallCount()).To(BeZero())
			})
		})

		Context("when a container already exists for the session", func() {
			var fakeContainer *wfakes.FakeContainer

			BeforeEach(func() {
				fakeContainer = new(wfakes.FakeContainer)
				workerClient.FindContainerForIdentifierReturns(fakeContainer, true, nil)
			})

			It("does not error and returns a resource", func() {
				Expect(initErr).NotTo(HaveOccurred())
				Expect(initResource).NotTo(BeNil())
			})

			It("does not create a container", func() {
				Expect(workerClient.SatisfyingCallCount()).To(BeZero())
				Expect(workerClient.CreateContainerCallCount()).To(BeZero())
			})

			It("returns them all as missing sources", func() {
				Expect(missingSources).To(ConsistOf("source-1-name", "source-2-name", "source-3-name"))
			})
		})
	})
})
