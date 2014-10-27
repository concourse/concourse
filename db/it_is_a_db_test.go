package db_test

import (
	"fmt"
	"time"

	"github.com/concourse/atc/config"
	. "github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func itIsADB() {
	BeforeEach(func() {
		err := db.RegisterJob("some-job")
		Ω(err).ShouldNot(HaveOccurred())

		err = db.RegisterJob("some-job")
		Ω(err).ShouldNot(HaveOccurred())

		err = db.RegisterResource("some-resource")
		Ω(err).ShouldNot(HaveOccurred())

		err = db.RegisterResource("some-resource")
		Ω(err).ShouldNot(HaveOccurred())

		err = db.RegisterJob("some-other-job")
		Ω(err).ShouldNot(HaveOccurred())

		err = db.RegisterResource("some-other-resource")
		Ω(err).ShouldNot(HaveOccurred())
	})

	It("initially reports zero builds for a job", func() {
		builds, err := db.GetAllJobBuilds("some-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(builds).Should(BeEmpty())
	})

	It("initially has no current build for a job", func() {
		_, err := db.GetCurrentBuild("some-job")
		Ω(err).Should(HaveOccurred())
	})

	Context("when a build is created for a job", func() {
		var build1 Build

		BeforeEach(func() {
			var err error

			build1, err = db.CreateJobBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(build1.ID).ShouldNot(BeZero())
			Ω(build1.JobName).Should(Equal("some-job"))
			Ω(build1.Name).Should(Equal("1"))
			Ω(build1.Status).Should(Equal(StatusPending))
		})

		It("can be read back as the same object", func() {
			gotBuild, err := db.GetBuild(build1.ID)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(gotBuild).Should(Equal(build1))
		})

		It("becomes the current build", func() {
			currentBuild, err := db.GetCurrentBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(currentBuild).Should(Equal(build1))
		})

		It("becomes the next pending build", func() {
			nextPending, _, err := db.GetNextPendingBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(nextPending).Should(Equal(build1))
		})

		It("is not reported as a started build", func() {
			Ω(db.GetAllStartedBuilds()).Should(BeEmpty())
		})

		It("is returned in the job's builds", func() {
			Ω(db.GetAllJobBuilds("some-job")).Should(ConsistOf([]Build{build1}))
		})

		It("is returned in the set of all builds", func() {
			Ω(db.GetAllBuilds()).Should(Equal([]Build{build1}))
		})

		Context("when scheduled", func() {
			BeforeEach(func() {
				scheduled, err := db.ScheduleBuild(build1.ID, false)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(scheduled).Should(BeTrue())
			})

			It("remains the current build", func() {
				currentBuild, err := db.GetCurrentBuild("some-job")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(currentBuild).Should(Equal(build1))
			})

			It("is no longer the next pending build", func() {
				_, _, err := db.GetNextPendingBuild("some-job")
				Ω(err).Should(HaveOccurred())
			})
		})

		Context("when started", func() {
			var expectedStartedBuild1 Build

			BeforeEach(func() {
				started, err := db.StartBuild(build1.ID, "some-guid", "some-endpoint")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(started).Should(BeTrue())

				expectedStartedBuild1 = build1
				expectedStartedBuild1.Status = StatusStarted
				expectedStartedBuild1.Guid = "some-guid"
				expectedStartedBuild1.Endpoint = "some-endpoint"
			})

			It("saves the updated status, and the guid and endpoint", func() {
				currentBuild, err := db.GetCurrentBuild("some-job")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(currentBuild).Should(Equal(expectedStartedBuild1))
			})

			It("is not reported as a started build", func() {
				Ω(db.GetAllStartedBuilds()).Should(ConsistOf([]Build{expectedStartedBuild1}))
			})
		})

		Context("when the status is updated", func() {
			var expectedSucceededBuild1 Build

			BeforeEach(func() {
				err := db.SaveBuildStatus(build1.ID, StatusSucceeded)
				Ω(err).ShouldNot(HaveOccurred())

				expectedSucceededBuild1 = build1
				expectedSucceededBuild1.Status = StatusSucceeded
			})

			It("is reflected through getting the build", func() {
				currentBuild, err := db.GetCurrentBuild("some-job")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(currentBuild).Should(Equal(expectedSucceededBuild1))
			})
		})

		Context("and another is created for the same job", func() {
			var build2 Build

			BeforeEach(func() {
				var err error
				build2, err = db.CreateJobBuild("some-job")
				Ω(err).ShouldNot(HaveOccurred())

				Ω(build2.ID).ShouldNot(BeZero())
				Ω(build2.ID).ShouldNot(Equal(build1.ID))
				Ω(build2.Name).Should(Equal("2"))
				Ω(build2.Status).Should(Equal(StatusPending))
			})

			It("can also be read back as the same object", func() {
				gotBuild, err := db.GetBuild(build2.ID)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(gotBuild).Should(Equal(build2))
			})

			It("is returned in the job's builds, before the rest", func() {
				Ω(db.GetAllJobBuilds("some-job")).Should(Equal([]Build{
					build2,
					build1,
				}))
			})

			It("is returned in the set of all builds, before the rest", func() {
				Ω(db.GetAllBuilds()).Should(Equal([]Build{build2, build1}))
			})

			Describe("the first build", func() {
				It("remains the next pending build", func() {
					nextPending, _, err := db.GetNextPendingBuild("some-job")
					Ω(err).ShouldNot(HaveOccurred())
					Ω(nextPending).Should(Equal(build1))
				})

				It("remains the current build", func() {
					currentBuild, err := db.GetCurrentBuild("some-job")
					Ω(err).ShouldNot(HaveOccurred())
					Ω(currentBuild).Should(Equal(build1))
				})
			})

			Context("when the first build is scheduled", func() {
				BeforeEach(func() {
					scheduled, err := db.ScheduleBuild(build1.ID, false)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeTrue())
				})

				Describe("the first build", func() {
					It("remains the current build", func() {
						currentBuild, err := db.GetCurrentBuild("some-job")
						Ω(err).ShouldNot(HaveOccurred())
						Ω(currentBuild).Should(Equal(build1))
					})
				})

				Describe("the second build", func() {
					It("becomes the next pending build", func() {
						nextPending, _, err := db.GetNextPendingBuild("some-job")
						Ω(err).ShouldNot(HaveOccurred())
						Ω(nextPending).Should(Equal(build2))
					})
				})
			})
		})

		Context("and another is created for a different job", func() {
			var otherJobBuild Build

			BeforeEach(func() {
				var err error

				otherJobBuild, err = db.CreateJobBuild("some-other-job")
				Ω(err).ShouldNot(HaveOccurred())

				Ω(otherJobBuild.ID).ShouldNot(BeZero())
				Ω(otherJobBuild.Name).Should(Equal("1"))
				Ω(otherJobBuild.Status).Should(Equal(StatusPending))
			})

			It("shows up in its job's builds", func() {
				Ω(db.GetAllJobBuilds("some-other-job")).Should(Equal([]Build{otherJobBuild}))
			})

			It("does not show up in the first build's job's builds", func() {
				Ω(db.GetAllJobBuilds("some-job")).Should(Equal([]Build{build1}))
			})

			It("is returned in the set of all builds, before the rest", func() {
				Ω(db.GetAllBuilds()).Should(Equal([]Build{otherJobBuild, build1}))
			})
		})
	})

	It("saves events correctly", func() {
		build, err := db.CreateJobBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(build.Name).Should(Equal("1"))

		By("initially returning zero-values for events and last ID")
		events, err := db.GetBuildEvents(build.ID)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(events).Should(BeEmpty())

		lastID, err := db.GetLastBuildEventID(build.ID)
		Ω(err).Should(HaveOccurred())
		Ω(lastID).Should(Equal(0))

		By("saving them in order and knowing the last ID")
		err = db.SaveBuildEvent(build.ID, BuildEvent{
			ID:      0,
			Type:    "log",
			Payload: "some ",
		})
		Ω(err).ShouldNot(HaveOccurred())

		lastID, err = db.GetLastBuildEventID(build.ID)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(lastID).Should(Equal(0))

		err = db.SaveBuildEvent(build.ID, BuildEvent{
			ID:      1,
			Type:    "log",
			Payload: "log",
		})
		Ω(err).ShouldNot(HaveOccurred())

		lastID, err = db.GetLastBuildEventID(build.ID)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(lastID).Should(Equal(1))

		By("being idempotent")
		err = db.SaveBuildEvent(build.ID, BuildEvent{
			ID:      1,
			Type:    "log",
			Payload: "log",
		})
		Ω(err).ShouldNot(HaveOccurred())

		lastID, err = db.GetLastBuildEventID(build.ID)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(lastID).Should(Equal(1))

		events, err = db.GetBuildEvents(build.ID)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(events).Should(Equal([]BuildEvent{
			BuildEvent{
				ID:      0,
				Type:    "log",
				Payload: "some ",
			},
			BuildEvent{
				ID:      1,
				Type:    "log",
				Payload: "log",
			},
		}))
	})

	It("can create one-off builds with increasing names", func() {
		oneOff, err := db.CreateOneOffBuild()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(oneOff.ID).ShouldNot(BeZero())
		Ω(oneOff.JobName).Should(BeZero())
		Ω(oneOff.Name).Should(Equal("1"))
		Ω(oneOff.Status).Should(Equal(StatusPending))

		oneOffGot, err := db.GetBuild(oneOff.ID)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(oneOffGot).Should(Equal(oneOff))

		jobBuild, err := db.CreateJobBuild("some-other-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(jobBuild.Name).Should(Equal("1"))

		nextOneOff, err := db.CreateOneOffBuild()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(nextOneOff.ID).ShouldNot(BeZero())
		Ω(nextOneOff.ID).ShouldNot(Equal(oneOff.ID))
		Ω(nextOneOff.JobName).Should(BeZero())
		Ω(nextOneOff.Name).Should(Equal("2"))
		Ω(nextOneOff.Status).Should(Equal(StatusPending))

		allBuilds, err := db.GetAllBuilds()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(allBuilds).Should(Equal([]Build{nextOneOff, jobBuild, oneOff}))
	})

	It("can save a build's start/end timestamps", func() {
		build, err := db.CreateJobBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(build.Name).Should(Equal("1"))

		startTime := time.Now()
		endTime := startTime.Add(time.Second)

		err = db.SaveBuildStartTime(build.ID, startTime)
		Ω(err).ShouldNot(HaveOccurred())

		err = db.SaveBuildEndTime(build.ID, endTime)
		Ω(err).ShouldNot(HaveOccurred())

		build, err = db.GetBuild(build.ID)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(build.StartTime.Unix()).Should(Equal(startTime.Unix()))
		Ω(build.EndTime.Unix()).Should(Equal(endTime.Unix()))
	})

	It("can report a job's latest running and finished builds", func() {
		finished, next, err := db.GetJobFinishedAndNextBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())

		Ω(next).Should(BeNil())
		Ω(finished).Should(BeNil())

		finishedBuild, err := db.CreateJobBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())

		err = db.SaveBuildStatus(finishedBuild.ID, StatusSucceeded)
		Ω(err).ShouldNot(HaveOccurred())

		finished, next, err = db.GetJobFinishedAndNextBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())

		Ω(next).Should(BeNil())
		Ω(finished.ID).Should(Equal(finishedBuild.ID))

		nextBuild, err := db.CreateJobBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())

		started, err := db.StartBuild(nextBuild.ID, "some-guid", "some-other-endpoint")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(started).Should(BeTrue())

		finished, next, err = db.GetJobFinishedAndNextBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())

		Ω(next.ID).Should(Equal(nextBuild.ID))
		Ω(finished.ID).Should(Equal(finishedBuild.ID))

		anotherRunningBuild, err := db.CreateJobBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())

		finished, next, err = db.GetJobFinishedAndNextBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())

		Ω(next.ID).Should(Equal(nextBuild.ID)) // not anotherRunningBuild
		Ω(finished.ID).Should(Equal(finishedBuild.ID))

		started, err = db.StartBuild(anotherRunningBuild.ID, "some-guid", "some-other-endpoint")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(started).Should(BeTrue())

		finished, next, err = db.GetJobFinishedAndNextBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())

		Ω(next.ID).Should(Equal(nextBuild.ID)) // not anotherRunningBuild
		Ω(finished.ID).Should(Equal(finishedBuild.ID))

		err = db.SaveBuildStatus(nextBuild.ID, StatusSucceeded)
		Ω(err).ShouldNot(HaveOccurred())

		finished, next, err = db.GetJobFinishedAndNextBuild("some-job")
		Ω(err).ShouldNot(HaveOccurred())

		Ω(next.ID).Should(Equal(anotherRunningBuild.ID))
		Ω(finished.ID).Should(Equal(nextBuild.ID))
	})

	Describe("locking", func() {
		It("can be done for resource checking", func() {
			lock, err := db.AcquireResourceCheckingLock()
			Ω(err).ShouldNot(HaveOccurred())

			secondLockCh := make(chan Lock, 1)

			go func() {
				defer GinkgoRecover()

				secondLock, err := db.AcquireResourceCheckingLock()
				Ω(err).ShouldNot(HaveOccurred())

				secondLockCh <- secondLock
			}()

			Consistently(secondLockCh).ShouldNot(Receive())

			err = lock.Release()
			Ω(err).ShouldNot(HaveOccurred())

			var secondLock Lock
			Eventually(secondLockCh).Should(Receive(&secondLock))

			err = secondLock.Release()
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("can be done for build scheduling", func() {
			lock, err := db.AcquireBuildSchedulingLock()
			Ω(err).ShouldNot(HaveOccurred())

			secondLockCh := make(chan Lock, 1)

			go func() {
				defer GinkgoRecover()

				secondLock, err := db.AcquireBuildSchedulingLock()
				Ω(err).ShouldNot(HaveOccurred())

				secondLockCh <- secondLock
			}()

			Consistently(secondLockCh).ShouldNot(Receive())

			err = lock.Release()
			Ω(err).ShouldNot(HaveOccurred())

			var secondLock Lock
			Eventually(secondLockCh).Should(Receive(&secondLock))

			err = secondLock.Release()
			Ω(err).ShouldNot(HaveOccurred())
		})
	})

	Describe("saving build inputs", func() {
		buildMetadata := []MetadataField{
			{
				Name:  "meta1",
				Value: "value1",
			},
			{
				Name:  "meta2",
				Value: "value2",
			},
		}

		vr1 := VersionedResource{
			Name:     "some-resource",
			Type:     "some-type",
			Source:   Source{"some": "source"},
			Version:  Version{"ver": "1"},
			Metadata: buildMetadata,
		}

		vr2 := VersionedResource{
			Name:    "some-other-resource",
			Type:    "some-type",
			Source:  Source{"some": "other-source"},
			Version: Version{"ver": "2"},
		}

		It("saves build's inputs and outputs as versioned resources", func() {
			build, err := db.CreateJobBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildInput(build.ID, vr1)
			Ω(err).ShouldNot(HaveOccurred())

			_, err = db.GetJobBuildForInputs("some-job", VersionedResources{vr1, vr2})
			Ω(err).Should(HaveOccurred())

			err = db.SaveBuildInput(build.ID, vr2)
			Ω(err).ShouldNot(HaveOccurred())

			foundBuild, err := db.GetJobBuildForInputs("some-job", VersionedResources{vr1, vr2})
			Ω(err).ShouldNot(HaveOccurred())
			Ω(foundBuild).Should(Equal(build))

			err = db.SaveBuildOutput(build.ID, vr1)
			Ω(err).ShouldNot(HaveOccurred())

			modifiedVR2 := vr2
			modifiedVR2.Version = Version{"ver": "3"}

			err = db.SaveBuildOutput(build.ID, modifiedVR2)
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildOutput(build.ID, vr2)
			Ω(err).ShouldNot(HaveOccurred())

			inputs, outputs, err := db.GetBuildResources(build.ID)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(inputs).Should(ConsistOf([]BuildInput{
				{VersionedResource: vr1, FirstOccurrence: true},
				{VersionedResource: vr2, FirstOccurrence: true},
			}))
			Ω(outputs).Should(ConsistOf([]BuildOutput{
				{VersionedResource: modifiedVR2},
			}))

			duplicateBuild, err := db.CreateJobBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildInput(duplicateBuild.ID, vr1)
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildInput(duplicateBuild.ID, vr2)
			Ω(err).ShouldNot(HaveOccurred())

			inputs, _, err = db.GetBuildResources(duplicateBuild.ID)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(inputs).Should(ConsistOf([]BuildInput{
				{VersionedResource: vr1, FirstOccurrence: false},
				{VersionedResource: vr2, FirstOccurrence: false},
			}))

			newBuildInOtherJob, err := db.CreateJobBuild("some-other-job")
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildInput(newBuildInOtherJob.ID, vr1)
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildInput(newBuildInOtherJob.ID, vr2)
			Ω(err).ShouldNot(HaveOccurred())

			inputs, _, err = db.GetBuildResources(newBuildInOtherJob.ID)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(inputs).Should(ConsistOf([]BuildInput{
				{VersionedResource: vr1, FirstOccurrence: true},
				{VersionedResource: vr2, FirstOccurrence: true},
			}))
		})

		It("updates metadata of existing inputs resources", func() {
			build, err := db.CreateJobBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildInput(build.ID, vr2)
			Ω(err).ShouldNot(HaveOccurred())

			inputs, _, err := db.GetBuildResources(build.ID)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(inputs).Should(ConsistOf([]BuildInput{
				{VersionedResource: vr2, FirstOccurrence: true},
			}))

			withMetadata := vr2
			withMetadata.Metadata = buildMetadata

			err = db.SaveBuildInput(build.ID, withMetadata)
			Ω(err).ShouldNot(HaveOccurred())

			inputs, _, err = db.GetBuildResources(build.ID)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(inputs).Should(ConsistOf([]BuildInput{
				{VersionedResource: withMetadata, FirstOccurrence: true},
			}))
		})

		It("can be done on build creation", func() {
			inputs := VersionedResources{vr1, vr2}

			pending, err := db.CreateJobBuildWithInputs("some-job", inputs)
			Ω(err).ShouldNot(HaveOccurred())

			foundBuild, err := db.GetJobBuildForInputs("some-job", inputs)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(foundBuild).Should(Equal(pending))

			nextPending, pendingInputs, err := db.GetNextPendingBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(nextPending).Should(Equal(pending))
			Ω(pendingInputs).Should(Equal(inputs))
		})
	})

	Describe("saving versioned resources", func() {
		It("updates the latest versioned resource", func() {
			vr1 := VersionedResource{
				Name:    "some-resource",
				Type:    "some-type",
				Source:  Source{"some": "source"},
				Version: Version{"version": "1"},
				Metadata: []MetadataField{
					{Name: "meta1", Value: "value1"},
				},
			}

			vr2 := VersionedResource{
				Name:    "some-resource",
				Type:    "some-type",
				Source:  Source{"some": "source"},
				Version: Version{"version": "2"},
				Metadata: []MetadataField{
					{Name: "meta2", Value: "value2"},
				},
			}

			vr3 := VersionedResource{
				Name:    "some-resource",
				Type:    "some-type",
				Source:  Source{"some": "source"},
				Version: Version{"version": "3"},
				Metadata: []MetadataField{
					{Name: "meta3", Value: "value3"},
				},
			}

			err := db.SaveVersionedResource(vr1)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(db.GetLatestVersionedResource("some-resource")).Should(Equal(vr1))

			err = db.SaveVersionedResource(vr2)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(db.GetLatestVersionedResource("some-resource")).Should(Equal(vr2))

			err = db.SaveVersionedResource(vr3)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(db.GetLatestVersionedResource("some-resource")).Should(Equal(vr3))
		})

		It("overwrites the existing source and metadata for the same version", func() {
			vr := VersionedResource{
				Name:    "some-resource",
				Type:    "some-type",
				Source:  Source{"some": "source"},
				Version: Version{"version": "1"},
				Metadata: []MetadataField{
					{Name: "meta1", Value: "value1"},
				},
			}

			err := db.SaveVersionedResource(vr)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(db.GetLatestVersionedResource("some-resource")).Should(Equal(vr))

			modified := vr
			modified.Source["additional"] = "data"
			modified.Metadata[0].Value = "modified-value1"

			err = db.SaveVersionedResource(modified)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(db.GetLatestVersionedResource("some-resource")).Should(Equal(modified))
		})
	})

	Describe("determining the inputs for a job", func() {
		It("ensures that versions from jobs mentioned in two input's 'passed' sections came from the same builds", func() {
			err := db.RegisterJob("job-1")
			Ω(err).ShouldNot(HaveOccurred())

			err = db.RegisterJob("job-2")
			Ω(err).ShouldNot(HaveOccurred())

			err = db.RegisterJob("shared-job")
			Ω(err).ShouldNot(HaveOccurred())

			err = db.RegisterResource("resource-1")
			Ω(err).ShouldNot(HaveOccurred())

			err = db.RegisterResource("resource-2")
			Ω(err).ShouldNot(HaveOccurred())

			j1b1, err := db.CreateJobBuild("job-1")
			Ω(err).ShouldNot(HaveOccurred())

			j2b1, err := db.CreateJobBuild("job-2")
			Ω(err).ShouldNot(HaveOccurred())

			sb1, err := db.CreateJobBuild("shared-job")
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildOutput(sb1.ID, VersionedResource{
				Name:    "resource-1",
				Type:    "some-type",
				Version: Version{"v": "r1-common-to-shared-and-j1"},
			})
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildOutput(sb1.ID, VersionedResource{
				Name:    "resource-2",
				Type:    "some-type",
				Version: Version{"v": "r2-common-to-shared-and-j2"},
			})
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildOutput(j1b1.ID, VersionedResource{
				Name:    "resource-1",
				Type:    "some-type",
				Version: Version{"v": "r1-common-to-shared-and-j1"},
			})
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildOutput(j2b1.ID, VersionedResource{
				Name:    "resource-2",
				Type:    "some-type",
				Version: Version{"v": "r2-common-to-shared-and-j2"},
			})
			Ω(err).ShouldNot(HaveOccurred())

			Ω(db.GetLatestInputVersions([]config.Input{
				{
					Resource: "resource-1",
					Passed:   []string{"shared-job", "job-1"},
				},
				{
					Resource: "resource-2",
					Passed:   []string{"shared-job", "job-2"},
				},
			})).Should(Equal(VersionedResources{
				{
					Name:    "resource-1",
					Type:    "some-type",
					Version: Version{"v": "r1-common-to-shared-and-j1"},
				},
				{
					Name:    "resource-2",
					Type:    "some-type",
					Version: Version{"v": "r2-common-to-shared-and-j2"},
				},
			}))

			sb2, err := db.CreateJobBuild("shared-job")
			Ω(err).ShouldNot(HaveOccurred())

			j1b2, err := db.CreateJobBuild("job-1")
			Ω(err).ShouldNot(HaveOccurred())

			j2b2, err := db.CreateJobBuild("job-2")
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildOutput(sb2.ID, VersionedResource{
				Name:    "resource-1",
				Type:    "some-type",
				Version: Version{"v": "new-r1-common-to-shared-and-j1"},
			})
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildOutput(sb2.ID, VersionedResource{
				Name:    "resource-2",
				Type:    "some-type",
				Version: Version{"v": "new-r2-common-to-shared-and-j2"},
			})
			Ω(err).ShouldNot(HaveOccurred())

			err = db.SaveBuildOutput(j1b2.ID, VersionedResource{
				Name:    "resource-1",
				Type:    "some-type",
				Version: Version{"v": "new-r1-common-to-shared-and-j1"},
			})
			Ω(err).ShouldNot(HaveOccurred())

			// do NOT save resource-2 as an output of job-2

			Ω(db.GetLatestInputVersions([]config.Input{
				{
					Resource: "resource-1",
					Passed:   []string{"shared-job", "job-1"},
				},
				{
					Resource: "resource-2",
					Passed:   []string{"shared-job", "job-2"},
				},
			})).Should(Equal(VersionedResources{
				{
					Name:    "resource-1",
					Type:    "some-type",
					Version: Version{"v": "r1-common-to-shared-and-j1"},
				},
				{
					Name:    "resource-2",
					Type:    "some-type",
					Version: Version{"v": "r2-common-to-shared-and-j2"},
				},
			}))

			// now save the output of resource-2 job-2
			err = db.SaveBuildOutput(j2b2.ID, VersionedResource{
				Name:    "resource-2",
				Type:    "some-type",
				Version: Version{"v": "new-r2-common-to-shared-and-j2"},
			})
			Ω(err).ShouldNot(HaveOccurred())

			Ω(db.GetLatestInputVersions([]config.Input{
				{
					Resource: "resource-1",
					Passed:   []string{"shared-job", "job-1"},
				},
				{
					Resource: "resource-2",
					Passed:   []string{"shared-job", "job-2"},
				},
			})).Should(Equal(VersionedResources{
				{
					Name:    "resource-1",
					Type:    "some-type",
					Version: Version{"v": "new-r1-common-to-shared-and-j1"},
				},
				{
					Name:    "resource-2",
					Type:    "some-type",
					Version: Version{"v": "new-r2-common-to-shared-and-j2"},
				},
			}))

			// save newer versions; should be new latest
			for i := 0; i < 10; i++ {
				version := fmt.Sprintf("version-%d", i+1)

				err = db.SaveBuildOutput(sb1.ID, VersionedResource{
					Name:    "resource-1",
					Type:    "some-type",
					Version: Version{"v": version + "-r1-common-to-shared-and-j1"},
				})
				Ω(err).ShouldNot(HaveOccurred())

				err = db.SaveBuildOutput(sb1.ID, VersionedResource{
					Name:    "resource-2",
					Type:    "some-type",
					Version: Version{"v": version + "-r2-common-to-shared-and-j2"},
				})
				Ω(err).ShouldNot(HaveOccurred())

				err = db.SaveBuildOutput(j1b1.ID, VersionedResource{
					Name:    "resource-1",
					Type:    "some-type",
					Version: Version{"v": version + "-r1-common-to-shared-and-j1"},
				})
				Ω(err).ShouldNot(HaveOccurred())

				err = db.SaveBuildOutput(j2b1.ID, VersionedResource{
					Name:    "resource-2",
					Type:    "some-type",
					Version: Version{"v": version + "-r2-common-to-shared-and-j2"},
				})
				Ω(err).ShouldNot(HaveOccurred())

				Ω(db.GetLatestInputVersions([]config.Input{
					{
						Resource: "resource-1",
						Passed:   []string{"shared-job", "job-1"},
					},
					{
						Resource: "resource-2",
						Passed:   []string{"shared-job", "job-2"},
					},
				})).Should(Equal(VersionedResources{
					{
						Name:    "resource-1",
						Type:    "some-type",
						Version: Version{"v": version + "-r1-common-to-shared-and-j1"},
					},
					{
						Name:    "resource-2",
						Type:    "some-type",
						Version: Version{"v": version + "-r2-common-to-shared-and-j2"},
					},
				}))
			}
		})
	})

	Context("when the first build is created", func() {
		var firstBuild Build

		var job string

		BeforeEach(func() {
			var err error

			job = "some-job"

			firstBuild, err = db.CreateJobBuild(job)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(firstBuild.Name).Should(Equal("1"))
			Ω(firstBuild.Status).Should(Equal(StatusPending))
		})

		Context("and then aborted", func() {
			BeforeEach(func() {
				err := db.SaveBuildStatus(firstBuild.ID, StatusAborted)
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("changes the state to aborted", func() {
				build, err := db.GetJobBuild(job, firstBuild.Name)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(build.Status).Should(Equal(StatusAborted))
			})

			Describe("scheduling the build", func() {
				It("fails", func() {
					scheduled, err := db.ScheduleBuild(firstBuild.ID, false)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeFalse())
				})
			})
		})

		Context("and then scheduled", func() {
			BeforeEach(func() {
				scheduled, err := db.ScheduleBuild(firstBuild.ID, false)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(scheduled).Should(BeTrue())
			})

			Context("and then aborted", func() {
				BeforeEach(func() {
					err := db.SaveBuildStatus(firstBuild.ID, StatusAborted)
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("changes the state to aborted", func() {
					build, err := db.GetJobBuild(job, firstBuild.Name)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(build.Status).Should(Equal(StatusAborted))
				})

				Describe("starting the build", func() {
					It("fails", func() {
						started, err := db.StartBuild(firstBuild.ID, "some-guid", "some-endpoint")
						Ω(err).ShouldNot(HaveOccurred())
						Ω(started).Should(BeFalse())
					})
				})
			})
		})

		Describe("scheduling the build", func() {
			It("succeeds", func() {
				scheduled, err := db.ScheduleBuild(firstBuild.ID, false)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(scheduled).Should(BeTrue())
			})

			Context("serially", func() {
				It("succeeds", func() {
					scheduled, err := db.ScheduleBuild(firstBuild.ID, true)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeTrue())
				})
			})
		})

		Context("and a second build is created", func() {
			var secondBuild Build

			Context("for a different job", func() {
				BeforeEach(func() {
					var err error

					secondBuild, err = db.CreateJobBuild("some-other-job")
					Ω(err).ShouldNot(HaveOccurred())
					Ω(secondBuild.Name).Should(Equal("1"))
					Ω(secondBuild.Status).Should(Equal(StatusPending))
				})

				Describe("scheduling the second build", func() {
					It("succeeds", func() {
						scheduled, err := db.ScheduleBuild(secondBuild.ID, false)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeTrue())
					})

					Describe("serially", func() {
						It("succeeds", func() {
							scheduled, err := db.ScheduleBuild(secondBuild.ID, true)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(scheduled).Should(BeTrue())
						})
					})
				})
			})

			Context("for the same job", func() {
				BeforeEach(func() {
					var err error

					secondBuild, err = db.CreateJobBuild(job)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(secondBuild.Name).Should(Equal("2"))
					Ω(secondBuild.Status).Should(Equal(StatusPending))
				})

				Describe("scheduling the second build", func() {
					It("succeeds", func() {
						scheduled, err := db.ScheduleBuild(secondBuild.ID, false)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeTrue())
					})

					Describe("serially", func() {
						It("fails", func() {
							scheduled, err := db.ScheduleBuild(secondBuild.ID, true)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(scheduled).Should(BeFalse())
						})
					})
				})

				Describe("after the first build schedules", func() {
					BeforeEach(func() {
						scheduled, err := db.ScheduleBuild(firstBuild.ID, false)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeTrue())
					})

					Context("when the second build is scheduled serially", func() {
						It("fails", func() {
							scheduled, err := db.ScheduleBuild(secondBuild.ID, true)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(scheduled).Should(BeFalse())
						})
					})

					for _, s := range []Status{StatusSucceeded, StatusFailed, StatusErrored} {
						status := s

						Context("and the first build's status changes to "+string(status), func() {
							BeforeEach(func() {
								err := db.SaveBuildStatus(firstBuild.ID, status)
								Ω(err).ShouldNot(HaveOccurred())
							})

							Context("and the second build is scheduled serially", func() {
								It("succeeds", func() {
									scheduled, err := db.ScheduleBuild(secondBuild.ID, true)
									Ω(err).ShouldNot(HaveOccurred())
									Ω(scheduled).Should(BeTrue())
								})
							})
						})
					}
				})

				Describe("after the first build is aborted", func() {
					BeforeEach(func() {
						err := db.SaveBuildStatus(firstBuild.ID, StatusAborted)
						Ω(err).ShouldNot(HaveOccurred())
					})

					Context("when the second build is scheduled serially", func() {
						It("succeeds", func() {
							scheduled, err := db.ScheduleBuild(secondBuild.ID, true)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(scheduled).Should(BeTrue())
						})
					})
				})

				Context("and a third build is created", func() {
					var thirdBuild Build

					BeforeEach(func() {
						var err error

						thirdBuild, err = db.CreateJobBuild(job)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(thirdBuild.Name).Should(Equal("3"))
						Ω(thirdBuild.Status).Should(Equal(StatusPending))
					})

					Context("and the first build finishes", func() {
						BeforeEach(func() {
							err := db.SaveBuildStatus(firstBuild.ID, StatusSucceeded)
							Ω(err).ShouldNot(HaveOccurred())
						})

						Context("and the third build is scheduled serially", func() {
							It("fails, as it would have jumped the queue", func() {
								scheduled, err := db.ScheduleBuild(thirdBuild.ID, true)
								Ω(err).ShouldNot(HaveOccurred())
								Ω(scheduled).Should(BeFalse())
							})
						})
					})

					Context("and then scheduled", func() {
						It("succeeds", func() {
							scheduled, err := db.ScheduleBuild(thirdBuild.ID, false)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(scheduled).Should(BeTrue())
						})

						Describe("serially", func() {
							It("fails", func() {
								scheduled, err := db.ScheduleBuild(thirdBuild.ID, true)
								Ω(err).ShouldNot(HaveOccurred())
								Ω(scheduled).Should(BeFalse())
							})
						})
					})
				})
			})
		})
	})
}
