package resource_test

import (
	"errors"

	"github.com/concourse/atc/resource/fakes"
	"github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/fakes"
	"github.com/concourse/baggageclaim"
	bfakes "github.com/concourse/baggageclaim/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/concourse/atc/resource"
)

type testMetadata []string

func (m testMetadata) Env() []string { return m }

var _ = Describe("Tracker", func() {
	var (
		tracker Tracker
	)

	var session = Session{
		ID: worker.Identifier{
			Name: "some-name",
		},
		Ephemeral: true,
	}

	BeforeEach(func() {
		tracker = NewTracker(workerClient)
	})

	Describe("Init", func() {
		var (
			logger   *lagertest.TestLogger
			metadata Metadata = testMetadata{"a=1", "b=2"}

			initType ResourceType

			initResource Resource
			initErr      error
		)

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("test")
			initType = "type1"

			workerClient.CreateContainerReturns(fakeContainer, nil)
		})

		JustBeforeEach(func() {
			initResource, initErr = tracker.Init(logger, metadata, session, initType, []string{"resource", "tags"})
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
				_, id, spec := workerClient.CreateContainerArgsForCall(0)

				Expect(id).To(Equal(session.ID))
				resourceSpec := spec.(worker.ResourceTypeContainerSpec)

				Expect(resourceSpec.Type).To(Equal(string(initType)))
				Expect(resourceSpec.Env).To(Equal([]string{"a=1", "b=2"}))
				Expect(resourceSpec.Ephemeral).To(Equal(true))
				Expect(resourceSpec.Tags).To(ConsistOf("resource", "tags"))
				Expect(resourceSpec.Cache).To(BeZero())
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

	Describe("InitWithCache", func() {
		var (
			logger   *lagertest.TestLogger
			metadata Metadata = testMetadata{"a=1", "b=2"}

			initType        ResourceType
			cacheIdentifier *fakes.FakeCacheIdentifier

			initResource Resource
			initCache    Cache
			initErr      error
		)

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("test")
			initType = "type1"
			cacheIdentifier = new(fakes.FakeCacheIdentifier)
		})

		JustBeforeEach(func() {
			initResource, initCache, initErr = tracker.InitWithCache(
				logger,
				metadata,
				session,
				initType,
				[]string{"resource", "tags"},
				cacheIdentifier,
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
					workerClient.SatisfyingReturns(satisfyingWorker, nil)

					satisfyingWorker.CreateContainerReturns(fakeContainer, nil)
				})

				Context("when the worker supports volume management", func() {
					var fakeBaggageclaimClient *bfakes.FakeClient

					BeforeEach(func() {
						fakeBaggageclaimClient = new(bfakes.FakeClient)
						satisfyingWorker.VolumeManagerReturns(fakeBaggageclaimClient, true)
					})

					Context("when the cache is already present", func() {
						var foundVolume *bfakes.FakeVolume

						BeforeEach(func() {
							foundVolume = new(bfakes.FakeVolume)
							foundVolume.HandleReturns("found-volume-handle")
							cacheIdentifier.FindOnReturns(foundVolume, true, nil)
						})

						It("does not error and returns a resource", func() {
							Expect(initErr).NotTo(HaveOccurred())
							Expect(initResource).NotTo(BeNil())
						})

						It("chose the worker satisfying the resource type and tags", func() {
							Expect(workerClient.SatisfyingArgsForCall(0)).To(Equal(worker.WorkerSpec{
								ResourceType: "type1",
								Tags:         []string{"resource", "tags"},
							}))
						})

						It("located it on the correct worker", func() {
							Expect(cacheIdentifier.FindOnCallCount()).To(Equal(1))
							_, baggageclaimClient := cacheIdentifier.FindOnArgsForCall(0)
							Expect(baggageclaimClient).To(Equal(fakeBaggageclaimClient))
						})

						It("creates the container with the cache volume", func() {
							_, id, spec := satisfyingWorker.CreateContainerArgsForCall(0)

							Expect(id).To(Equal(session.ID))
							resourceSpec := spec.(worker.ResourceTypeContainerSpec)

							Expect(resourceSpec.Type).To(Equal(string(initType)))
							Expect(resourceSpec.Env).To(Equal([]string{"a=1", "b=2"}))
							Expect(resourceSpec.Ephemeral).To(Equal(true))
							Expect(resourceSpec.Tags).To(ConsistOf("resource", "tags"))
							Expect(resourceSpec.Cache).To(Equal(worker.VolumeMount{
								Volume:    foundVolume,
								MountPath: "/tmp/build/get",
							}))
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
						var createdVolume *bfakes.FakeVolume

						BeforeEach(func() {
							cacheIdentifier.FindOnReturns(nil, false, nil)

							createdVolume = new(bfakes.FakeVolume)
							createdVolume.HandleReturns("created-volume-handle")

							cacheIdentifier.CreateOnReturns(createdVolume, nil)
						})

						It("does not error and returns a resource", func() {
							Expect(initErr).NotTo(HaveOccurred())
							Expect(initResource).NotTo(BeNil())
						})

						It("chose the worker satisfying the resource type and tags", func() {
							Expect(workerClient.SatisfyingArgsForCall(0)).To(Equal(worker.WorkerSpec{
								ResourceType: "type1",
								Tags:         []string{"resource", "tags"},
							}))
						})

						It("created the volume on the right worker", func() {
							Expect(cacheIdentifier.CreateOnCallCount()).To(Equal(1))
							_, baggageclaimClient := cacheIdentifier.CreateOnArgsForCall(0)
							Expect(baggageclaimClient).To(Equal(fakeBaggageclaimClient))
						})

						It("creates the container with the created cache volume", func() {
							_, id, spec := satisfyingWorker.CreateContainerArgsForCall(0)

							Expect(id).To(Equal(session.ID))
							resourceSpec := spec.(worker.ResourceTypeContainerSpec)

							Expect(resourceSpec.Type).To(Equal(string(initType)))
							Expect(resourceSpec.Env).To(Equal([]string{"a=1", "b=2"}))
							Expect(resourceSpec.Ephemeral).To(Equal(true))
							Expect(resourceSpec.Tags).To(ConsistOf("resource", "tags"))
							Expect(resourceSpec.Cache).To(Equal(worker.VolumeMount{
								Volume:    createdVolume,
								MountPath: "/tmp/build/get",
							}))
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

				Context("when the worker does not support volume management", func() {
					BeforeEach(func() {
						satisfyingWorker.VolumeManagerReturns(nil, false)
					})

					It("creates a container", func() {
						_, id, spec := satisfyingWorker.CreateContainerArgsForCall(0)

						Expect(id).To(Equal(session.ID))
						resourceSpec := spec.(worker.ResourceTypeContainerSpec)

						Expect(resourceSpec.Type).To(Equal(string(initType)))
						Expect(resourceSpec.Env).To(Equal([]string{"a=1", "b=2"}))
						Expect(resourceSpec.Ephemeral).To(Equal(true))
						Expect(resourceSpec.Tags).To(ConsistOf("resource", "tags"))
						Expect(resourceSpec.Cache).To(BeZero())
					})

					Context("when creating the container fails", func() {
						disaster := errors.New("oh no!")

						BeforeEach(func() {
							satisfyingWorker.CreateContainerReturns(nil, disaster)
						})

						It("returns the error and no resource", func() {
							Expect(initErr).To(Equal(disaster))
							Expect(initResource).To(BeNil())
						})
					})
				})
			})

			Context("when no worker satisfies the spec", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					workerClient.SatisfyingReturns(nil, disaster)
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

			Context("when the container has a cache volume", func() {
				var cacheVolume *bfakes.FakeVolume

				BeforeEach(func() {
					cacheVolume = new(bfakes.FakeVolume)
					fakeContainer.VolumesReturns([]baggageclaim.Volume{cacheVolume})
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
								Expect(initCache.IsInitialized()).To(BeTrue())
							})
						})

						Context("when the volume has no initialized property", func() {
							BeforeEach(func() {
								cacheVolume.PropertiesReturns(baggageclaim.VolumeProperties{}, nil)
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
								cacheVolume.PropertiesReturns(nil, disaster)
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
								err := initCache.Initialize()
								Expect(err).To(Equal(disaster))
							})
						})
					})
				})
			})

			Context("when the container has no volumes", func() {
				BeforeEach(func() {
					fakeContainer.VolumesReturns([]baggageclaim.Volume{})
				})

				Describe("the cache", func() {
					It("is not initialized", func() {
						initialized, err := initCache.IsInitialized()
						Expect(initialized).To(BeFalse())
						Expect(err).ToNot(HaveOccurred())
					})

					It("does a no-op initialize", func() {
						Expect(initCache.Initialize()).To(Succeed())
					})
				})
			})
		})
	})
})
