package scheduler_test

import (
	"errors"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/concourse/concourse/atc/scheduler"
	"github.com/concourse/concourse/atc/scheduler/algorithm/algorithmfakes"
	"github.com/concourse/concourse/atc/scheduler/schedulerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Scheduler", func() {
	var (
		fakeInputMapper  *algorithmfakes.FakeInputMapper
		fakeBuildStarter *schedulerfakes.FakeBuildStarter

		scheduler *Scheduler

		disaster error
	)

	BeforeEach(func() {
		fakeInputMapper = new(algorithmfakes.FakeInputMapper)
		fakeBuildStarter = new(schedulerfakes.FakeBuildStarter)

		scheduler = &Scheduler{
			InputMapper:  fakeInputMapper,
			BuildStarter: fakeBuildStarter,
		}

		disaster = errors.New("bad thing")
	})

	Describe("Schedule", func() {
		var (
			versionsDB             *db.VersionsDB
			fakeJob                *dbfakes.FakeJob
			fakeResource           *dbfakes.FakeResource
			scheduleErr            error
			versionedResourceTypes atc.VersionedResourceTypes
		)

		BeforeEach(func() {
			versionedResourceTypes = atc.VersionedResourceTypes{
				{
					ResourceType: atc.ResourceType{Name: "some-resource-type"},
					Version:      atc.Version{"some": "version"},
				},
			}

			fakeResource = new(dbfakes.FakeResource)
			fakeResource.NameReturns("some-resource")
		})

		JustBeforeEach(func() {
			versionsDB = &db.VersionsDB{JobIDs: map[string]int{"j1": 1}}

			var waiter interface{ Wait() }

			scheduleErr = scheduler.Schedule(
				lagertest.NewTestLogger("test"),
				versionsDB,
				fakeJob,
				db.Resources{fakeResource},
				versionedResourceTypes,
			)
			if waiter != nil {
				waiter.Wait()
			}
		})

		Context("when the job has no inputs", func() {
			BeforeEach(func() {
				fakeJob = new(dbfakes.FakeJob)
				fakeJob.NameReturns("some-job-1")
			})

			Context("when mapping the inputs fails", func() {
				BeforeEach(func() {
					fakeInputMapper.MapInputsReturns(nil, false, disaster)
				})

				It("returns the error", func() {
					Expect(scheduleErr).To(Equal(disaster))
				})
			})

			Context("when mapping the inputs succeeds", func() {
				var expectedInputMapping db.InputMapping

				BeforeEach(func() {
					expectedInputMapping = map[string]db.InputResult{
						"input-1": db.InputResult{
							Input: &db.AlgorithmInput{
								AlgorithmVersion: db.AlgorithmVersion{
									ResourceID: 1,
									Version:    db.ResourceVersion("1"),
								},
								FirstOccurrence: true,
							},
						},
					}

					fakeInputMapper.MapInputsReturns(expectedInputMapping, true, nil)
				})

				It("mapped the inputs", func() {
					Expect(fakeInputMapper.MapInputsCallCount()).To(Equal(1))
					actualVersionsDB, actualJob, _ := fakeInputMapper.MapInputsArgsForCall(0)
					Expect(actualVersionsDB).To(Equal(versionsDB))
					Expect(actualJob.Name()).To(Equal(fakeJob.Name()))
				})

				Context("when saving the next input mapping fails", func() {
					BeforeEach(func() {
						fakeJob.SaveNextInputMappingReturns(disaster)
					})

					It("returns the error", func() {
						Expect(scheduleErr).To(Equal(disaster))
					})
				})

				Context("when saving the next input mapping succeeds", func() {
					BeforeEach(func() {
						fakeJob.SaveNextInputMappingReturns(nil)
					})

					It("saved the next input mapping", func() {
						Expect(fakeJob.SaveNextInputMappingCallCount()).To(Equal(1))
						actualInputMapping, resolved := fakeJob.SaveNextInputMappingArgsForCall(0)
						Expect(actualInputMapping).To(Equal(expectedInputMapping))
						Expect(resolved).To(BeTrue())
					})

					Context("when getting the full next build inputs fails", func() {
						BeforeEach(func() {
							fakeJob.GetFullNextBuildInputsReturns(nil, false, disaster)
						})

						It("returns the error", func() {
							Expect(scheduleErr).To(Equal(disaster))
						})
					})

					Context("when getting the full next build inputs succeeds", func() {
						BeforeEach(func() {
							fakeJob.GetFullNextBuildInputsReturns([]db.BuildInput{}, true, nil)
						})

						Context("when starting pending builds for job fails", func() {
							BeforeEach(func() {
								fakeBuildStarter.TryStartPendingBuildsForJobReturns(disaster)
							})

							It("returns the error", func() {
								Expect(scheduleErr).To(Equal(disaster))
							})

							It("started all pending builds", func() {
								Expect(fakeBuildStarter.TryStartPendingBuildsForJobCallCount()).To(Equal(1))
								_, actualJob, actualResources, actualResourceTypes := fakeBuildStarter.TryStartPendingBuildsForJobArgsForCall(0)
								Expect(actualJob.Name()).To(Equal(fakeJob.Name()))
								Expect(actualResources).To(Equal(db.Resources{fakeResource}))
								Expect(actualResourceTypes).To(Equal(versionedResourceTypes))
							})
						})

						Context("when starting all pending builds succeeds", func() {
							BeforeEach(func() {
								fakeBuildStarter.TryStartPendingBuildsForJobReturns(nil)
							})

							It("returns no error", func() {
								Expect(scheduleErr).NotTo(HaveOccurred())
							})

							It("didn't create a pending build", func() {
								//TODO: create a positive test case for this
								Expect(fakeJob.EnsurePendingBuildExistsCallCount()).To(BeZero())
							})
						})
					})
				})

				It("didn't mark the job as having new inputs", func() {
					Expect(fakeJob.SetHasNewInputsCallCount()).To(BeZero())
				})
			})
		})

		Context("when the job has one trigger: true input", func() {
			BeforeEach(func() {
				fakeJob = new(dbfakes.FakeJob)
				fakeJob.NameReturns("some-job")
				fakeJob.ConfigReturns(atc.JobConfig{
					Plan: atc.PlanSequence{
						{Get: "a", Trigger: true},
						{Get: "b", Trigger: false},
					},
				})

				fakeBuildStarter.TryStartPendingBuildsForJobReturns(nil)
				fakeJob.SaveNextInputMappingReturns(nil)
			})

			Context("when no input mapping is found", func() {
				BeforeEach(func() {
					fakeInputMapper.MapInputsReturns(db.InputMapping{}, false, nil)
				})

				It("starts all pending builds and returns no error", func() {
					Expect(fakeBuildStarter.TryStartPendingBuildsForJobCallCount()).To(Equal(1))
					Expect(scheduleErr).NotTo(HaveOccurred())
				})

				It("didn't create a pending build", func() {
					Expect(fakeJob.EnsurePendingBuildExistsCallCount()).To(BeZero())
				})

				It("didn't mark the job as having new inputs", func() {
					Expect(fakeJob.SetHasNewInputsCallCount()).To(BeZero())
				})
			})

			Context("when no first occurrence input has trigger: true", func() {
				BeforeEach(func() {
					fakeJob.GetFullNextBuildInputsReturns([]db.BuildInput{
						{
							Name:            "a",
							Version:         atc.Version{"ref": "v1"},
							ResourceID:      11,
							FirstOccurrence: false,
						},
						{
							Name:            "b",
							Version:         atc.Version{"ref": "v2"},
							ResourceID:      12,
							FirstOccurrence: true,
						},
					}, true, nil)
				})

				It("starts all pending builds and returns no error", func() {
					Expect(fakeBuildStarter.TryStartPendingBuildsForJobCallCount()).To(Equal(1))
					Expect(scheduleErr).NotTo(HaveOccurred())
				})

				It("didn't create a pending build", func() {
					Expect(fakeJob.EnsurePendingBuildExistsCallCount()).To(BeZero())
				})

				Context("when the job does not have new inputs since before", func() {
					BeforeEach(func() {
						fakeJob.HasNewInputsReturns(false)
					})

					Context("when marking job as having new input fails", func() {
						BeforeEach(func() {
							fakeJob.SetHasNewInputsReturns(disaster)
						})

						It("returns the error", func() {
							Expect(scheduleErr).To(Equal(disaster))
						})
					})

					Context("when marking job as having new input succeeds", func() {
						BeforeEach(func() {
							fakeJob.SetHasNewInputsReturns(nil)
						})

						It("did the needful", func() {
							Expect(fakeJob.SetHasNewInputsCallCount()).To(Equal(1))
							Expect(fakeJob.SetHasNewInputsArgsForCall(0)).To(Equal(true))
						})
					})
				})

				Context("when the job has new inputs since before", func() {
					BeforeEach(func() {
						fakeJob.HasNewInputsReturns(true)
					})

					It("doesn't mark the job as having new inputs", func() {
						Expect(fakeJob.SetHasNewInputsCallCount()).To(BeZero())
					})
				})
			})

			Context("when a first occurrence input has trigger: true", func() {
				BeforeEach(func() {
					fakeJob.GetFullNextBuildInputsReturns([]db.BuildInput{
						{
							Name:            "a",
							Version:         atc.Version{"ref": "v1"},
							ResourceID:      11,
							FirstOccurrence: true,
						},
						{
							Name:            "b",
							Version:         atc.Version{"ref": "v2"},
							ResourceID:      12,
							FirstOccurrence: false,
						},
					}, true, nil)
				})

				Context("when creating a pending build fails", func() {
					BeforeEach(func() {
						fakeJob.EnsurePendingBuildExistsReturns(disaster)
					})

					It("returns the error", func() {
						Expect(scheduleErr).To(Equal(disaster))
					})

					It("created a pending build for the right job", func() {
						Expect(fakeJob.EnsurePendingBuildExistsCallCount()).To(Equal(1))
					})
				})

				Context("when creating a pending build succeeds", func() {
					BeforeEach(func() {
						fakeJob.EnsurePendingBuildExistsReturns(nil)
					})

					It("starts all pending builds and returns no error", func() {
						Expect(fakeBuildStarter.TryStartPendingBuildsForJobCallCount()).To(Equal(1))
						Expect(scheduleErr).NotTo(HaveOccurred())
					})
				})
			})

			Context("when no first occurrence", func() {
				BeforeEach(func() {
					fakeJob.GetFullNextBuildInputsReturns([]db.BuildInput{
						{
							Name:            "a",
							Version:         atc.Version{"ref": "v1"},
							ResourceID:      11,
							FirstOccurrence: false,
						},
						{
							Name:            "b",
							Version:         atc.Version{"ref": "v2"},
							ResourceID:      12,
							FirstOccurrence: false,
						},
					}, true, nil)
				})

				Context("when job had new inputs", func() {
					BeforeEach(func() {
						fakeJob.HasNewInputsReturns(true)
					})

					It("marks the job as not having new inputs", func() {
						Expect(fakeJob.SetHasNewInputsCallCount()).To(Equal(1))
						Expect(fakeJob.SetHasNewInputsArgsForCall(0)).To(Equal(false))
					})
				})

				Context("when job did not have new inputs", func() {
					BeforeEach(func() {
						fakeJob.HasNewInputsReturns(false)
					})

					It("doesn't mark the the job as not having new inputs again", func() {
						Expect(fakeJob.SetHasNewInputsCallCount()).To(Equal(0))
					})
				})
			})
		})
	})
})
