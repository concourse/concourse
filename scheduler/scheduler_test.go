package scheduler_test

import (
	"errors"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/dbng/dbngfakes"
	. "github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/scheduler/inputmapper/inputmapperfakes"
	"github.com/concourse/atc/scheduler/schedulerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Scheduler", func() {
	var (
		fakePipeline     *dbngfakes.FakePipeline
		fakeInputMapper  *inputmapperfakes.FakeInputMapper
		fakeBuildStarter *schedulerfakes.FakeBuildStarter
		fakeScanner      *schedulerfakes.FakeScanner

		scheduler *Scheduler

		disaster error
	)

	BeforeEach(func() {
		fakePipeline = new(dbngfakes.FakePipeline)
		fakeInputMapper = new(inputmapperfakes.FakeInputMapper)
		fakeBuildStarter = new(schedulerfakes.FakeBuildStarter)
		fakeScanner = new(schedulerfakes.FakeScanner)

		scheduler = &Scheduler{
			Pipeline:     fakePipeline,
			InputMapper:  fakeInputMapper,
			BuildStarter: fakeBuildStarter,
			Scanner:      fakeScanner,
		}

		disaster = errors.New("bad thing")
	})

	Describe("Schedule", func() {
		var (
			versionsDB             *algorithm.VersionsDB
			jobConfigs             atc.JobConfigs
			nextPendingBuilds      []dbng.Build
			nextPendingBuildsJob1  []dbng.Build
			nextPendingBuildsJob2  []dbng.Build
			scheduleErr            error
			versionedResourceTypes atc.VersionedResourceTypes
		)

		BeforeEach(func() {
			nextPendingBuilds = []dbng.Build{new(dbngfakes.FakeBuild)}
			nextPendingBuildsJob1 = []dbng.Build{new(dbngfakes.FakeBuild), new(dbngfakes.FakeBuild)}
			nextPendingBuildsJob2 = []dbng.Build{new(dbngfakes.FakeBuild)}
			fakePipeline.GetAllPendingBuildsReturns(map[string][]dbng.Build{
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
		})

		JustBeforeEach(func() {
			versionsDB = &algorithm.VersionsDB{JobIDs: map[string]int{"j1": 1}}

			var waiter Waiter
			_, scheduleErr = scheduler.Schedule(
				lagertest.NewTestLogger("test"),
				versionsDB,
				jobConfigs,
				atc.ResourceConfigs{{Name: "some-resource"}},
				versionedResourceTypes,
			)
			if waiter != nil {
				waiter.Wait()
			}
		})

		Context("when the job has no inputs", func() {
			BeforeEach(func() {
				jobConfigs = atc.JobConfigs{
					{Name: "some-job-1"},
					{Name: "some-job-2"},
				}
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
					_, actualVersionsDB, actualJobConfig := fakeInputMapper.SaveNextInputMappingArgsForCall(0)
					Expect(actualVersionsDB).To(Equal(versionsDB))
					Expect(actualJobConfig).To(Equal(jobConfigs[0]))

					_, actualVersionsDB, actualJobConfig = fakeInputMapper.SaveNextInputMappingArgsForCall(1)
					Expect(actualVersionsDB).To(Equal(versionsDB))
					Expect(actualJobConfig).To(Equal(jobConfigs[1]))
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
						Expect(actualJob).To(Equal(jobConfigs[0]))
						Expect(actualResources).To(Equal(atc.ResourceConfigs{{Name: "some-resource"}}))
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
						Expect(fakePipeline.EnsurePendingBuildExistsCallCount()).To(BeZero())
					})
				})
			})
		})

		Context("when the job has one trigger: true input", func() {
			BeforeEach(func() {
				jobConfigs = atc.JobConfigs{
					{
						Name: "some-job",
						Plan: atc.PlanSequence{
							{Get: "a", Trigger: true},
							{Get: "b", Trigger: false},
						},
					},
				}

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
					Expect(fakePipeline.EnsurePendingBuildExistsCallCount()).To(BeZero())
				})
			})

			Context("when no first occurrence input has trigger: true", func() {
				BeforeEach(func() {
					fakeInputMapper.SaveNextInputMappingReturns(algorithm.InputMapping{
						"a": algorithm.InputVersion{VersionID: 1, FirstOccurrence: false},
						"b": algorithm.InputVersion{VersionID: 2, FirstOccurrence: true},
					}, nil)
				})

				It("starts all pending builds and returns no error", func() {
					Expect(fakeBuildStarter.TryStartPendingBuildsForJobCallCount()).To(Equal(1))
					Expect(scheduleErr).NotTo(HaveOccurred())
				})

				It("didn't create a pending build", func() {
					Expect(fakePipeline.EnsurePendingBuildExistsCallCount()).To(BeZero())
				})
			})

			Context("when a first occurrence input has trigger: true", func() {
				BeforeEach(func() {
					fakeInputMapper.SaveNextInputMappingReturns(algorithm.InputMapping{
						"a": algorithm.InputVersion{VersionID: 1, FirstOccurrence: true},
						"b": algorithm.InputVersion{VersionID: 2, FirstOccurrence: false},
					}, nil)
				})

				Context("when creating a pending build fails", func() {
					BeforeEach(func() {
						fakePipeline.EnsurePendingBuildExistsReturns(disaster)
					})

					It("returns the error", func() {
						Expect(scheduleErr).To(Equal(disaster))
					})

					It("created a pending build for the right job", func() {
						Expect(fakePipeline.EnsurePendingBuildExistsCallCount()).To(Equal(1))
						Expect(fakePipeline.EnsurePendingBuildExistsArgsForCall(0)).To(Equal("some-job"))
					})
				})

				Context("when creating a pending build succeeds", func() {
					BeforeEach(func() {
						fakePipeline.EnsurePendingBuildExistsReturns(nil)
					})

					It("starts all pending builds and returns no error", func() {
						Expect(fakeBuildStarter.TryStartPendingBuildsForJobCallCount()).To(Equal(1))
						Expect(scheduleErr).NotTo(HaveOccurred())
					})
				})
			})
		})
	})

	Describe("TriggerImmediately", func() {
		var (
			jobConfig         atc.JobConfig
			triggeredBuild    dbng.Build
			triggerErr        error
			nextPendingBuilds []dbng.Build
		)

		JustBeforeEach(func() {
			jobConfig = atc.JobConfig{Name: "some-job", Plan: atc.PlanSequence{{Get: "input-1"}, {Get: "input-2"}}}

			var waiter Waiter
			triggeredBuild, waiter, triggerErr = scheduler.TriggerImmediately(
				lagertest.NewTestLogger("test"),
				jobConfig,
				atc.ResourceConfigs{{Name: "some-resource"}},
				atc.VersionedResourceTypes{
					{
						ResourceType: atc.ResourceType{Name: "some-resource-type"},
						Version:      atc.Version{"some": "version"},
					},
				},
			)
			if waiter != nil {
				waiter.Wait()
			}
		})

		Context("when creating the build fails", func() {
			BeforeEach(func() {
				fakePipeline.CreateJobBuildReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(triggerErr).To(Equal(disaster))
			})
		})

		Context("when creating the build succeeds", func() {
			var createdBuild *dbngfakes.FakeBuild

			BeforeEach(func() {
				createdBuild = new(dbngfakes.FakeBuild)
				createdBuild.IsManuallyTriggeredReturns(true)
				fakePipeline.CreateJobBuildReturns(createdBuild, nil)
			})

			It("tried to create a build for the right job", func() {
				Expect(fakePipeline.CreateJobBuildCallCount()).To(Equal(1))
				Expect(fakePipeline.CreateJobBuildArgsForCall(0)).To(Equal("some-job"))
			})

			Context("when get pending builds for job fails", func() {
				BeforeEach(func() {
					fakePipeline.GetPendingBuildsForJobReturns(nil, disaster)
				})

				It("does not try to start pending builds for job", func() {
					Expect(fakeBuildStarter.TryStartPendingBuildsForJobCallCount()).To(Equal(0))
				})
			})

			Context("when get pending builds for job succeeds", func() {
				BeforeEach(func() {
					nextPendingBuilds = []dbng.Build{new(dbngfakes.FakeBuild)}
					fakePipeline.GetPendingBuildsForJobReturns(nextPendingBuilds, nil)
				})

				It("tried to get pending builds for the right job", func() {
					Expect(fakePipeline.GetPendingBuildsForJobCallCount()).To(Equal(1))
					Expect(fakePipeline.GetPendingBuildsForJobArgsForCall(0)).To(Equal("some-job"))
				})

				Context("when trying to start pending builds succeeds", func() {
					BeforeEach(func() {
						fakeBuildStarter.TryStartPendingBuildsForJobReturns(nil)
					})

					It("tries to start builds for the right job", func() {
						Expect(fakeBuildStarter.TryStartPendingBuildsForJobCallCount()).To(Equal(1))
						_, _, _, _, b := fakeBuildStarter.TryStartPendingBuildsForJobArgsForCall(0)
						Expect(b).To(Equal(nextPendingBuilds))
					})
				})
			})
		})
	})

	Describe("SaveNextInputMapping", func() {
		var saveErr error

		JustBeforeEach(func() {
			saveErr = scheduler.SaveNextInputMapping(lagertest.NewTestLogger("test"), atc.JobConfig{Name: "some-job"})
		})

		Context("when loading the versions DB fails", func() {
			BeforeEach(func() {
				fakePipeline.LoadVersionsDBReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(saveErr).To(Equal(disaster))
			})
		})

		Context("when loading the versions DB succeeds", func() {
			var versionsDB *algorithm.VersionsDB

			BeforeEach(func() {
				versionsDB = &algorithm.VersionsDB{JobIDs: map[string]int{"j1": 1}}
				fakePipeline.LoadVersionsDBReturns(versionsDB, nil)
			})

			Context("when saving the next input mapping fails", func() {
				BeforeEach(func() {
					fakeInputMapper.SaveNextInputMappingReturns(nil, disaster)
				})

				It("returns the error", func() {
					Expect(saveErr).To(Equal(disaster))
				})

				It("saved the next input mapping for the right job and versions", func() {
					Expect(fakeInputMapper.SaveNextInputMappingCallCount()).To(Equal(1))
					_, actualVersionsDB, actualJobConfig := fakeInputMapper.SaveNextInputMappingArgsForCall(0)
					Expect(actualVersionsDB).To(Equal(versionsDB))
					Expect(actualJobConfig).To(Equal(atc.JobConfig{Name: "some-job"}))
				})
			})

			Context("when saving the next input mapping succeeds", func() {
				BeforeEach(func() {
					fakeInputMapper.SaveNextInputMappingReturns(algorithm.InputMapping{
						"some-input": algorithm.InputVersion{VersionID: 1, FirstOccurrence: true},
					}, nil)
				})

				It("returns no error", func() {
					Expect(saveErr).NotTo(HaveOccurred())
				})
			})
		})
	})
})
