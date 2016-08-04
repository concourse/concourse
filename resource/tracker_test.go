package resource_test

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/resource/resourcefakes"
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

			workerClient.CreateContainerReturns(fakeContainer, nil)
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
			teamID         = 123
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
				teamID,
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
								TeamID:       teamID,
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
