package resource_test

import (
	"errors"

	"github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/concourse/atc/resource"
)

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
		workerClient.CreateContainerReturns(fakeContainer, nil)

		tracker = NewTracker(workerClient)
	})

	Describe("Init", func() {
		var (
			initType ResourceType

			initResource Resource
			initErr      error
		)

		BeforeEach(func() {
			initType = "type1"
		})

		JustBeforeEach(func() {
			initResource, initErr = tracker.Init(session, initType)
		})

		Context("when a container does not exist for the session", func() {
			BeforeEach(func() {
				workerClient.LookupReturns(nil, errors.New("nope"))
			})

			It("does not error and returns a resource", func() {
				Ω(initErr).ShouldNot(HaveOccurred())
				Ω(initResource).ShouldNot(BeNil())
			})

			It("creates a container with the resource's type, ephemeral information, and the session as the handle", func() {
				id, spec := workerClient.CreateContainerArgsForCall(0)

				Ω(id).Should(Equal(session.ID))
				Ω(spec).Should(Equal(worker.ResourceTypeContainerSpec{
					Type:      string(initType),
					Ephemeral: true,
				}))
			})

			Context("when creating the container fails", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					workerClient.CreateContainerReturns(nil, disaster)
				})

				It("returns the error and no resource", func() {
					Ω(initErr).Should(Equal(disaster))
					Ω(initResource).Should(BeNil())
				})
			})
		})

		Context("when a container already exists for the session", func() {
			var fakeContainer *wfakes.FakeContainer

			BeforeEach(func() {
				fakeContainer = new(wfakes.FakeContainer)
				workerClient.LookupReturns(fakeContainer, nil)
			})

			It("does not error and returns a resource", func() {
				Ω(initErr).ShouldNot(HaveOccurred())
				Ω(initResource).ShouldNot(BeNil())
			})

			It("does not create a container", func() {
				Ω(workerClient.CreateContainerCallCount()).Should(BeZero())
			})
		})
	})
})
