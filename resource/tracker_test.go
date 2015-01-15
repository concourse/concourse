package resource_test

import (
	"errors"

	"github.com/cloudfoundry-incubator/garden"
	wfakes "github.com/concourse/atc/worker/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/concourse/atc/resource"
)

const sessionID = "some-session-id"

var _ = Describe("Tracker", func() {
	var (
		resourceTypes ResourceMapping

		tracker Tracker
	)

	BeforeEach(func() {
		resourceTypes = ResourceMapping{
			"type1": "image1",
			"type2": "image2",
		}

		workerClient.CreateReturns(fakeContainer, nil)

		tracker = NewTracker(resourceTypes, workerClient)
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
			initResource, initErr = tracker.Init(sessionID, initType)
		})

		Context("when a container does not exist for the session", func() {
			BeforeEach(func() {
				workerClient.LookupReturns(nil, errors.New("nope"))
			})

			It("does not error and returns a resource", func() {
				Ω(initErr).ShouldNot(HaveOccurred())
				Ω(initResource).ShouldNot(BeNil())
			})

			It("creates a privileged container with the resource type's image, and the session as the handle", func() {
				Ω(workerClient.CreateArgsForCall(0)).Should(Equal(garden.ContainerSpec{
					Handle:     sessionID,
					RootFSPath: "image1",
					Privileged: true,
				}))
			})

			Context("when creating the container fails", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					workerClient.CreateReturns(nil, disaster)
				})

				It("returns the error and no resource", func() {
					Ω(initErr).Should(Equal(disaster))
					Ω(initResource).Should(BeNil())
				})
			})

			Context("with an unknown resource type", func() {
				BeforeEach(func() {
					initType = "bogus-type"
				})

				It("returns ErrUnknownResourceType", func() {
					Ω(initErr).Should(Equal(ErrUnknownResourceType))
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
				Ω(workerClient.CreateCallCount()).Should(BeZero())
			})
		})
	})
})
