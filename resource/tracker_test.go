package resource_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/workerfakes"

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
		trackerFactory := NewTrackerFactory()
		tracker = trackerFactory.TrackerFor(workerClient)
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
			teamID       = 123
		)

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("test")
			initType = "type1"
			delegate = new(wfakes.FakeImageFetchingDelegate)

			workerClient.CreateTaskContainerReturns(fakeContainer, nil)
		})

		JustBeforeEach(func() {
			initResource, initErr = tracker.Init(logger, metadata, session, initType, []string{"resource", "tags"}, teamID, customTypes, delegate)
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
				_, _, _, id, containerMetadata, spec, actualCustomTypes, _ := workerClient.CreateTaskContainerArgsForCall(0)

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
					workerClient.CreateTaskContainerReturns(nil, disaster)
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
				Expect(workerClient.CreateTaskContainerCallCount()).To(BeZero())
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
				Expect(workerClient.CreateTaskContainerCallCount()).To(BeZero())
			})
		})
	})
})
