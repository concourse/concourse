package worker_test

import (
	"errors"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	gfakes "github.com/cloudfoundry-incubator/garden/fakes"
	"github.com/concourse/atc"
	. "github.com/concourse/atc/worker"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/clock/fakeclock"
)

var _ = Describe("Worker", func() {
	var (
		fakeGardenClient *gfakes.FakeClient
		fakeClock        *fakeclock.FakeClock
		activeContainers int
		resourceTypes    []atc.WorkerResourceType
		platform         string
		tags             []string

		worker Worker
	)

	BeforeEach(func() {
		fakeGardenClient = new(gfakes.FakeClient)
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))
		activeContainers = 42
		resourceTypes = []atc.WorkerResourceType{
			{Type: "some-resource", Image: "some-resource-image"},
		}
		platform = "some-platform"
		tags = []string{"some", "tags"}
	})

	JustBeforeEach(func() {
		worker = NewGardenWorker(
			fakeGardenClient,
			fakeClock,
			activeContainers,
			resourceTypes,
			platform,
			tags,
		)
	})

	Describe("CreateContainer", func() {
		var (
			spec ContainerSpec

			createdContainer Container
			createErr        error
		)

		JustBeforeEach(func() {
			createdContainer, createErr = worker.CreateContainer("some-handle", spec)
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
							Handle:     "some-handle",
							RootFSPath: "some-resource-image",
							Privileged: true,
						}))
					})

					Context("if the container is marked as ephemeral", func() {
						BeforeEach(func() {
							spec = ResourceTypeContainerSpec{
								Type:      "some-resource",
								Ephemeral: true,
							}
						})

						It("adds an 'ephemeral' property to the container", func() {
							Ω(fakeGardenClient.CreateCallCount()).Should(Equal(1))
							Ω(fakeGardenClient.CreateArgsForCall(0)).Should(Equal(garden.ContainerSpec{
								Handle:     "some-handle",
								RootFSPath: "some-resource-image",
								Privileged: true,
								Properties: garden.Properties{
									"ephemeral": "true",
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

		Context("with a resource type container spec", func() {
			BeforeEach(func() {
				spec = ExecuteContainerSpec{
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
						Handle:     "some-handle",
						RootFSPath: "some-image",
						Privileged: true,
					}))
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

	Describe("Lookup", func() {
		var (
			handle string

			foundContainer Container
			lookupErr      error
		)

		BeforeEach(func() {
			handle = "some-handle"
		})

		JustBeforeEach(func() {
			foundContainer, lookupErr = worker.Lookup(handle)
		})

		Context("when the container can be found", func() {
			var fakeContainer *gfakes.FakeContainer

			BeforeEach(func() {
				fakeContainer = new(gfakes.FakeContainer)
				fakeContainer.HandleReturns(handle)

				fakeGardenClient.LookupReturns(fakeContainer, nil)
			})

			It("succeeds", func() {
				Ω(lookupErr).ShouldNot(HaveOccurred())
			})

			It("looks up the container with the Garden client", func() {
				Ω(fakeGardenClient.LookupCallCount()).Should(Equal(1))
				Ω(fakeGardenClient.LookupArgsForCall(0)).Should(Equal(handle))
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
			})
		})

		Context("when creating fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeGardenClient.LookupReturns(nil, disaster)
			})

			It("returns the error", func() {
				Ω(lookupErr).Should(Equal(disaster))
			})
		})
	})

	Describe("Satisfies", func() {
		Context("with an ExecuteContainerSpec", func() {
			var (
				spec      ExecuteContainerSpec
				satisfies bool
			)

			BeforeEach(func() {
				spec = ExecuteContainerSpec{}
			})

			JustBeforeEach(func() {
				satisfies = worker.Satisfies(spec)
			})

			Context("when the platform is compatible", func() {
				BeforeEach(func() {
					spec.Platform = "some-platform"
				})

				Context("when no tags are specified", func() {
					BeforeEach(func() {
						spec.Tags = nil
					})

					It("returns false", func() {
						Ω(satisfies).Should(BeFalse())
					})
				})

				Context("when the worker has no tags", func() {
					BeforeEach(func() {
						tags = []string{}
					})

					It("returns true", func() {
						Ω(satisfies).Should(BeTrue())
					})
				})

				Context("when all of the requested tags are present", func() {
					BeforeEach(func() {
						spec.Tags = []string{"some", "tags"}
					})

					It("returns true", func() {
						Ω(satisfies).Should(BeTrue())
					})
				})

				Context("when some of the requested tags are present", func() {
					BeforeEach(func() {
						spec.Tags = []string{"some"}
					})

					It("returns true", func() {
						Ω(satisfies).Should(BeTrue())
					})
				})

				Context("when any of the requested tags are not present", func() {
					BeforeEach(func() {
						spec.Tags = []string{"bogus", "tags"}
					})

					It("returns false", func() {
						Ω(satisfies).Should(BeFalse())
					})
				})
			})

			Context("when the platform is incompatible", func() {
				BeforeEach(func() {
					spec.Platform = "some-bogus-platform"
				})

				It("returns false", func() {
					Ω(satisfies).Should(BeFalse())
				})
			})
		})

		Context("with a ResourceTypeContainerSpec", func() {
			var (
				spec      ResourceTypeContainerSpec
				satisfies bool
			)

			BeforeEach(func() {
				spec = ResourceTypeContainerSpec{}
			})

			JustBeforeEach(func() {
				satisfies = worker.Satisfies(spec)
			})

			Context("when the type is supported by the worker", func() {
				BeforeEach(func() {
					spec = ResourceTypeContainerSpec{
						Type: "some-resource",
					}
				})

				It("returns true", func() {
					Ω(satisfies).Should(BeTrue())
				})
			})

			Context("when the type is not supported by the worker", func() {
				BeforeEach(func() {
					spec.Type = "some-other-resource"
				})

				It("returns false", func() {
					Ω(satisfies).Should(BeFalse())
				})
			})
		})
	})
})
