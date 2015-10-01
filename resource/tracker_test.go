package resource_test

import (
	"errors"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/fakes"
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
			ContainerIdentifier: db.ContainerIdentifier{
				Name: "some-name",
			},
		},
		Ephemeral: true,
	}

	BeforeEach(func() {
		workerClient.CreateContainerReturns(fakeContainer, nil)

		tracker = NewTracker(workerClient)
	})

	Describe("Init", func() {
		var (
			logger   *lagertest.TestLogger
			metadata Metadata = testMetadata{"a=1", "b=2"}

			initType    ResourceType
			volumeMount VolumeMount

			initResource Resource
			initErr      error
		)

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("test")
			initType = "type1"
			volumeMount = VolumeMount{
				Volume:    new(bfakes.FakeVolume),
				MountPath: "/some/mount/path",
			}
		})

		JustBeforeEach(func() {
			initResource, initErr = tracker.Init(logger, metadata, session, initType, []string{"resource", "tags"}, volumeMount)
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
				Expect(resourceSpec.Cache.Volume).To(Equal(volumeMount.Volume))
				Expect(resourceSpec.Cache.MountPath).To(Equal(volumeMount.MountPath))
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
})
