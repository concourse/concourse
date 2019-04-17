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
			fakeJob                *dbfakes.FakeJob
			fakeResource           *dbfakes.FakeResource
			nextPendingBuilds      []db.Build
			scheduleErr            error
			versionedResourceTypes atc.VersionedResourceTypes
		)

		BeforeEach(func() {
			nextPendingBuilds = []db.Build{new(dbfakes.FakeBuild)}
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
				fakeJob.GetPendingBuildsReturns(nextPendingBuilds, nil)
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

				It("saved the next input mapping", func() {
					Expect(fakeInputMapper.SaveNextInputMappingCallCount()).To(Equal(1))
					_, actualVersionsDB, actualJob, _ := fakeInputMapper.SaveNextInputMappingArgsForCall(0)
					Expect(actualVersionsDB).To(Equal(versionsDB))
					Expect(actualJob.Name()).To(Equal(fakeJob.Name()))
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
						_, actualJob, actualResources, actualResourceTypes, actualPendingBuilds := fakeBuildStarter.TryStartPendingBuildsForJobArgsForCall(0)
						Expect(actualJob.Name()).To(Equal(fakeJob.Name()))
						Expect(actualResources).To(Equal(db.Resources{fakeResource}))
						Expect(actualResourceTypes).To(Equal(versionedResourceTypes))
						Expect(actualPendingBuilds).To(Equal(nextPendingBuilds))
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
			})

			Context("when no first occurrence input has trigger: true", func() {
				BeforeEach(func() {
					fakeInputMapper.SaveNextInputMappingReturns(algorithm.InputMapping{
						"a": algorithm.InputSource{
							InputVersion:   algorithm.InputVersion{VersionID: 1, ResourceID: 11, FirstOccurrence: false},
							PassedBuildIDs: []int{},
						},
						"b": algorithm.InputSource{
							InputVersion:   algorithm.InputVersion{VersionID: 2, ResourceID: 12, FirstOccurrence: true},
							PassedBuildIDs: []int{},
						},
					}, nil)
				})

				It("starts all pending builds and returns no error", func() {
					Expect(fakeBuildStarter.TryStartPendingBuildsForJobCallCount()).To(Equal(1))
					Expect(scheduleErr).NotTo(HaveOccurred())
				})

				It("didn't create a pending build", func() {
					Expect(fakeJob.EnsurePendingBuildExistsCallCount()).To(BeZero())
				})
			})

			Context("when a first occurrence input has trigger: true", func() {
				BeforeEach(func() {
					fakeInputMapper.SaveNextInputMappingReturns(algorithm.InputMapping{
						"a": algorithm.InputSource{
							InputVersion:   algorithm.InputVersion{VersionID: 1, ResourceID: 11, FirstOccurrence: true},
							PassedBuildIDs: []int{},
						},
						"b": algorithm.InputSource{
							InputVersion:   algorithm.InputVersion{VersionID: 2, ResourceID: 12, FirstOccurrence: false},
							PassedBuildIDs: []int{},
						},
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
		})
	})
})
