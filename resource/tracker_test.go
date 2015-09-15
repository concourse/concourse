package resource_test

import (
	"errors"

	"github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/fakes"
	bfakes "github.com/concourse/baggageclaim/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

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
		workerClient.CreateContainerReturns(fakeContainer, nil)

		tracker = NewTracker(workerClient)
	})

	Describe("Init", func() {
		var (
			metadata Metadata = testMetadata{"a=1", "b=2"}

			initType    ResourceType
			volumeMount VolumeMount

			initResource Resource
			initErr      error
		)

		BeforeEach(func() {
			initType = "type1"
			volumeMount = VolumeMount{
				Volume:    new(bfakes.FakeVolume),
				MountPath: "/some/mount/path",
			}
		})

		JustBeforeEach(func() {
			initResource, initErr = tracker.Init(metadata, session, initType, []string{"resource", "tags"}, volumeMount)
		})

		Context("when a container does not exist for the session", func() {
			BeforeEach(func() {
				workerClient.FindContainerForIdentifierReturns(nil, false, nil)
			})

			It("does not error and returns a resource", func() {
				Ω(initErr).ShouldNot(HaveOccurred())
				Ω(initResource).ShouldNot(BeNil())
			})

			It("creates a container with the resource's type, env, ephemeral information, and the session as the handle", func() {
				id, spec := workerClient.CreateContainerArgsForCall(0)

				Ω(id).Should(Equal(session.ID))
				resourceSpec := spec.(worker.ResourceTypeContainerSpec)

				Ω(resourceSpec.Type).Should(Equal(string(initType)))
				Ω(resourceSpec.Env).Should(Equal([]string{"a=1", "b=2"}))
				Ω(resourceSpec.Ephemeral).Should(Equal(true))
				Ω(resourceSpec.Tags).Should(ConsistOf("resource", "tags"))
				Ω(resourceSpec.Volume).Should(Equal(volumeMount.Volume))
				Ω(resourceSpec.MountPath).Should(Equal(volumeMount.MountPath))
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

		Context("when looking up the container fails for some reason", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				workerClient.FindContainerForIdentifierReturns(nil, false, disaster)
			})

			It("returns the error and no resource", func() {
				Ω(initErr).Should(Equal(disaster))
				Ω(initResource).Should(BeNil())
			})

			It("does not create a container", func() {
				Ω(workerClient.CreateContainerCallCount()).Should(BeZero())
			})
		})

		Context("when a container already exists for the session", func() {
			var fakeContainer *wfakes.FakeContainer

			BeforeEach(func() {
				fakeContainer = new(wfakes.FakeContainer)
				workerClient.FindContainerForIdentifierReturns(fakeContainer, true, nil)
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
