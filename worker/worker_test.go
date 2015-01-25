package worker_test

import (
	"errors"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	gfakes "github.com/cloudfoundry-incubator/garden/fakes"
	. "github.com/concourse/atc/worker"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vito/clock/fakeclock"
)

var _ = Describe("Worker", func() {
	var (
		fakeGardenClient *gfakes.FakeClient
		fakeClock        *fakeclock.FakeClock
		activeContainers int

		worker Worker
	)

	BeforeEach(func() {
		fakeGardenClient = new(gfakes.FakeClient)
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))
		activeContainers = 42

		worker = NewGardenWorker(fakeGardenClient, fakeClock, activeContainers)
	})

	Describe("Create", func() {
		var (
			spec garden.ContainerSpec

			createdContainer Container
			createErr        error
		)

		BeforeEach(func() {
			spec = garden.ContainerSpec{
				RootFSPath: "some-rootfs-path",
			}
		})

		JustBeforeEach(func() {
			createdContainer, createErr = worker.Create(spec)
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
				Ω(fakeGardenClient.CreateArgsForCall(0)).Should(Equal(spec))
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
})
