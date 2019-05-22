package scheduler_test

import (
	"errors"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/algorithm"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/concourse/concourse/atc/scheduler"
	"github.com/concourse/concourse/atc/scheduler/inputmapper/inputmapperfakes"
	"github.com/concourse/concourse/atc/scheduler/schedulerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Scheduler", func() {
	var (
		fakePipeline     *dbfakes.FakePipeline
		fakeInputMapper  *inputmapperfakes.FakeInputMapper
		fakeBuildStarter *schedulerfakes.FakeBuildStarter

		scheduler *Scheduler

		disaster error
	)

	BeforeEach(func() {
		fakePipeline = new(dbfakes.FakePipeline)
		fakeInputMapper = new(inputmapperfakes.FakeInputMapper)
		fakeBuildStarter = new(schedulerfakes.FakeBuildStarter)

		scheduler = &Scheduler{
			Pipeline:     fakePipeline,
			InputMapper:  fakeInputMapper,
			BuildStarter: fakeBuildStarter,
		}

		disaster = errors.New("bad thing")
	})

	Describe("Schedule", func() {
		var (
			versionsDB             *algorithm.VersionsDB
			fakeJobs               []db.Job
			fakeJob                *dbfakes.FakeJob
			fakeJob2               *dbfakes.FakeJob
			fakeResource           *dbfakes.FakeResource
			nextPendingBuilds      []db.Build
			nextPendingBuildsJob1  []db.Build
			nextPendingBuildsJob2  []db.Build
			scheduleErr            error
			versionedResourceTypes atc.VersionedResourceTypes
		)

		BeforeEach(func() {
			nextPendingBuilds = []db.Build{new(dbfakes.FakeBuild)}
			nextPendingBuildsJob1 = []db.Build{new(dbfakes.FakeBuild), new(dbfakes.FakeBuild)}
			nextPendingBuildsJob2 = []db.Build{new(dbfakes.FakeBuild)}
			fakePipeline.GetAllPendingBuildsReturns(map[string][]db.Build{
				"some-job":   nextPendingBuilds,
				"some-job-1": nextPendingBuildsJob1,
				"some-job-2": nextPendingBuildsJob2,
			}, nil)

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
			versionsDB = &algorithm.VersionsDB{JobIDs: map[string]int{"j1": 1}}

			var waiter interface{ Wait() }

			_, scheduleErr = scheduler.Schedule(
				lagertest.NewTestLogger("test"),
				versionsDB,
				fakeJobs,
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

				fakeJob2 = new(dbfakes.FakeJob)
				fakeJob2.NameReturns("some-job-2")

				fakeJobs = []db.Job{fakeJob, fakeJob2}
			})

			Context("when saving the next input mapping fails", func() {
				BeforeEach(func() {
					fakeInputMapper.SaveNextInputMappingReturns(nil, disaster)
				})

				It("returns the error", func() {
					Expect(scheduleErr).To(Equal(disaster))
				})
			})

			Context("when saving the next input mapping succeeds", func() {
				BeforeEach(func() {
					fakeInputMapper.SaveNextInputMappingReturns(algorithm.InputMapping{}, nil)
				})

				It("saved the next input mapping for the right job and versions", func() {
					Expect(fakeInputMapper.SaveNextInputMappingCallCount()).To(Equal(2))
					_, actualVersionsDB, actualJob, _ := fakeInputMapper.SaveNextInputMappingArgsForCall(0)
					Expect(actualVersionsDB).To(Equal(versionsDB))
					Expect(actualJob.Name()).To(Equal(fakeJob.Name()))

					_, actualVersionsDB, actualJob, _ = fakeInputMapper.SaveNextInputMappingArgsForCall(1)
					Expect(actualVersionsDB).To(Equal(versionsDB))
					Expect(actualJob.Name()).To(Equal(fakeJob2.Name()))
				})

				Context("when starting pending builds for job fails", func() {
					BeforeEach(func() {
						fakeBuildStarter.TryStartPendingBuildsForJobReturns(disaster)
					})

					It("returns the error", func() {
						Expect(scheduleErr).To(Equal(disaster))
					})

					It("started all pending builds for the right job", func() {
						Expect(fakeBuildStarter.TryStartPendingBuildsForJobCallCount()).To(Equal(1))
						_, actualJob, actualResources, actualResourceTypes, actualPendingBuilds := fakeBuildStarter.TryStartPendingBuildsForJobArgsForCall(0)
						Expect(actualJob.Name()).To(Equal(fakeJob.Name()))
						Expect(actualResources).To(Equal(db.Resources{fakeResource}))
						Expect(actualResourceTypes).To(Equal(versionedResourceTypes))
						Expect(actualPendingBuilds).To(Equal(nextPendingBuildsJob1))
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
						Expect(fakeJob2.EnsurePendingBuildExistsCallCount()).To(BeZero())
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

				fakeJobs = []db.Job{fakeJob}

				fakeBuildStarter.TryStartPendingBuildsForJobReturns(nil)
			})

			Context("when no input mapping is found", func() {
				BeforeEach(func() {
					fakeInputMapper.SaveNextInputMappingReturns(algorithm.InputMapping{}, nil)
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
					fakeInputMapper.SaveNextInputMappingReturns(algorithm.InputMapping{
						"a": algorithm.InputVersion{VersionID: 1, ResourceID: 11, FirstOccurrence: false},
						"b": algorithm.InputVersion{VersionID: 2, ResourceID: 12, FirstOccurrence: true},
					}, nil)
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
					fakeInputMapper.SaveNextInputMappingReturns(algorithm.InputMapping{
						"a": algorithm.InputVersion{VersionID: 1, ResourceID: 11, FirstOccurrence: true},
						"b": algorithm.InputVersion{VersionID: 2, ResourceID: 12, FirstOccurrence: false},
					}, nil)
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
					fakeInputMapper.SaveNextInputMappingReturns(algorithm.InputMapping{
						"a": algorithm.InputVersion{VersionID: 1, ResourceID: 11, FirstOccurrence: false},
						"b": algorithm.InputVersion{VersionID: 2, ResourceID: 12, FirstOccurrence: false},
					}, nil)
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
