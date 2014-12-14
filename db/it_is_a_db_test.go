package db_test

import (
	"fmt"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type dbSharedBehaviorInput struct {
	db.DB
}

func dbSharedBehavior(database *dbSharedBehaviorInput) func() {
	return func() {
		It("initially reports zero builds for a job", func() {
			builds, err := database.GetAllJobBuilds("some-job")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(builds).Should(BeEmpty())
		})

		It("initially has no current build for a job", func() {
			_, err := database.GetCurrentBuild("some-job")
			Ω(err).Should(HaveOccurred())
		})

		Context("when a build is created for a job", func() {
			var build1 db.Build

			BeforeEach(func() {
				var err error

				build1, err = database.CreateJobBuild("some-job")
				Ω(err).ShouldNot(HaveOccurred())

				Ω(build1.ID).ShouldNot(BeZero())
				Ω(build1.JobName).Should(Equal("some-job"))
				Ω(build1.Name).Should(Equal("1"))
				Ω(build1.Status).Should(Equal(db.StatusPending))
			})

			It("can be read back as the same object", func() {
				gotBuild, err := database.GetBuild(build1.ID)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(gotBuild).Should(Equal(build1))
			})

			It("becomes the current build", func() {
				currentBuild, err := database.GetCurrentBuild("some-job")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(currentBuild).Should(Equal(build1))
			})

			It("becomes the next pending build", func() {
				nextPending, _, err := database.GetNextPendingBuild("some-job")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(nextPending).Should(Equal(build1))
			})

			It("is not reported as a started build", func() {
				Ω(database.GetAllStartedBuilds()).Should(BeEmpty())
			})

			It("is returned in the job's builds", func() {
				Ω(database.GetAllJobBuilds("some-job")).Should(ConsistOf([]db.Build{build1}))
			})

			It("is returned in the set of all builds", func() {
				Ω(database.GetAllBuilds()).Should(Equal([]db.Build{build1}))
			})

			Context("when scheduled", func() {
				BeforeEach(func() {
					scheduled, err := database.ScheduleBuild(build1.ID, false)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeTrue())
				})

				It("remains the current build", func() {
					currentBuild, err := database.GetCurrentBuild("some-job")
					Ω(err).ShouldNot(HaveOccurred())
					Ω(currentBuild).Should(Equal(build1))
				})

				It("remains the next pending build", func() {
					nextPending, _, err := database.GetNextPendingBuild("some-job")
					Ω(err).ShouldNot(HaveOccurred())
					Ω(nextPending).Should(Equal(build1))
				})
			})

			Context("when started", func() {
				var expectedStartedBuild1 db.Build

				BeforeEach(func() {
					started, err := database.StartBuild(build1.ID, "some-engine", "some-metadata")
					Ω(err).ShouldNot(HaveOccurred())
					Ω(started).Should(BeTrue())

					expectedStartedBuild1 = build1
					expectedStartedBuild1.Status = db.StatusStarted
					expectedStartedBuild1.Engine = "some-engine"
					expectedStartedBuild1.EngineMetadata = "some-metadata"
				})

				It("saves the updated status, and the engine and engine metadata", func() {
					currentBuild, err := database.GetCurrentBuild("some-job")
					Ω(err).ShouldNot(HaveOccurred())
					Ω(currentBuild).Should(Equal(expectedStartedBuild1))
				})

				It("is not reported as a started build", func() {
					Ω(database.GetAllStartedBuilds()).Should(ConsistOf([]db.Build{expectedStartedBuild1}))
				})
			})

			Context("when the status is updated", func() {
				var expectedSucceededBuild1 db.Build

				BeforeEach(func() {
					err := database.SaveBuildStatus(build1.ID, db.StatusSucceeded)
					Ω(err).ShouldNot(HaveOccurred())

					expectedSucceededBuild1 = build1
					expectedSucceededBuild1.Status = db.StatusSucceeded
				})

				It("is reflected through getting the build", func() {
					currentBuild, err := database.GetCurrentBuild("some-job")
					Ω(err).ShouldNot(HaveOccurred())
					Ω(currentBuild).Should(Equal(expectedSucceededBuild1))
				})
			})

			Context("and another is created for the same job", func() {
				var build2 db.Build

				BeforeEach(func() {
					var err error
					build2, err = database.CreateJobBuild("some-job")
					Ω(err).ShouldNot(HaveOccurred())

					Ω(build2.ID).ShouldNot(BeZero())
					Ω(build2.ID).ShouldNot(Equal(build1.ID))
					Ω(build2.Name).Should(Equal("2"))
					Ω(build2.Status).Should(Equal(db.StatusPending))
				})

				It("can also be read back as the same object", func() {
					gotBuild, err := database.GetBuild(build2.ID)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(gotBuild).Should(Equal(build2))
				})

				It("is returned in the job's builds, before the rest", func() {
					Ω(database.GetAllJobBuilds("some-job")).Should(Equal([]db.Build{
						build2,
						build1,
					}))
				})

				It("is returned in the set of all builds, before the rest", func() {
					Ω(database.GetAllBuilds()).Should(Equal([]db.Build{build2, build1}))
				})

				Describe("the first build", func() {
					It("remains the next pending build", func() {
						nextPending, _, err := database.GetNextPendingBuild("some-job")
						Ω(err).ShouldNot(HaveOccurred())
						Ω(nextPending).Should(Equal(build1))
					})

					It("remains the current build", func() {
						currentBuild, err := database.GetCurrentBuild("some-job")
						Ω(err).ShouldNot(HaveOccurred())
						Ω(currentBuild).Should(Equal(build1))
					})
				})
			})

			Context("and another is created for a different job", func() {
				var otherJobBuild db.Build

				BeforeEach(func() {
					var err error

					otherJobBuild, err = database.CreateJobBuild("some-other-job")
					Ω(err).ShouldNot(HaveOccurred())

					Ω(otherJobBuild.ID).ShouldNot(BeZero())
					Ω(otherJobBuild.Name).Should(Equal("1"))
					Ω(otherJobBuild.Status).Should(Equal(db.StatusPending))
				})

				It("shows up in its job's builds", func() {
					Ω(database.GetAllJobBuilds("some-other-job")).Should(Equal([]db.Build{otherJobBuild}))
				})

				It("does not show up in the first build's job's builds", func() {
					Ω(database.GetAllJobBuilds("some-job")).Should(Equal([]db.Build{build1}))
				})

				It("is returned in the set of all builds, before the rest", func() {
					Ω(database.GetAllBuilds()).Should(Equal([]db.Build{otherJobBuild, build1}))
				})
			})
		})

		It("saves events correctly", func() {
			build, err := database.CreateJobBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(build.Name).Should(Equal("1"))

			By("initially returning zero-values for events and last ID")
			events, err := database.GetBuildEvents(build.ID)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(events).Should(BeEmpty())

			lastID, err := database.GetLastBuildEventID(build.ID)
			Ω(err).Should(HaveOccurred())
			Ω(lastID).Should(Equal(0))

			By("saving them in order and knowing the last ID")
			err = database.SaveBuildEvent(build.ID, db.BuildEvent{
				ID:      0,
				Type:    "log",
				Payload: "some ",
			})
			Ω(err).ShouldNot(HaveOccurred())

			lastID, err = database.GetLastBuildEventID(build.ID)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(lastID).Should(Equal(0))

			err = database.SaveBuildEvent(build.ID, db.BuildEvent{
				ID:      1,
				Type:    "log",
				Payload: "log",
			})
			Ω(err).ShouldNot(HaveOccurred())

			lastID, err = database.GetLastBuildEventID(build.ID)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(lastID).Should(Equal(1))

			By("being idempotent")
			err = database.SaveBuildEvent(build.ID, db.BuildEvent{
				ID:      1,
				Type:    "log",
				Payload: "log",
			})
			Ω(err).ShouldNot(HaveOccurred())

			lastID, err = database.GetLastBuildEventID(build.ID)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(lastID).Should(Equal(1))

			events, err = database.GetBuildEvents(build.ID)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(events).Should(Equal([]db.BuildEvent{
				db.BuildEvent{
					ID:      0,
					Type:    "log",
					Payload: "some ",
				},
				db.BuildEvent{
					ID:      1,
					Type:    "log",
					Payload: "log",
				},
			}))
		})

		It("can create one-off builds with increasing names", func() {
			oneOff, err := database.CreateOneOffBuild()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(oneOff.ID).ShouldNot(BeZero())
			Ω(oneOff.JobName).Should(BeZero())
			Ω(oneOff.Name).Should(Equal("1"))
			Ω(oneOff.Status).Should(Equal(db.StatusPending))

			oneOffGot, err := database.GetBuild(oneOff.ID)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(oneOffGot).Should(Equal(oneOff))

			jobBuild, err := database.CreateJobBuild("some-other-job")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(jobBuild.Name).Should(Equal("1"))

			nextOneOff, err := database.CreateOneOffBuild()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(nextOneOff.ID).ShouldNot(BeZero())
			Ω(nextOneOff.ID).ShouldNot(Equal(oneOff.ID))
			Ω(nextOneOff.JobName).Should(BeZero())
			Ω(nextOneOff.Name).Should(Equal("2"))
			Ω(nextOneOff.Status).Should(Equal(db.StatusPending))

			allBuilds, err := database.GetAllBuilds()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(allBuilds).Should(Equal([]db.Build{nextOneOff, jobBuild, oneOff}))
		})

		It("can save a build's start/end timestamps", func() {
			build, err := database.CreateJobBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(build.Name).Should(Equal("1"))

			startTime := time.Now()
			endTime := startTime.Add(time.Second)

			err = database.SaveBuildStartTime(build.ID, startTime)
			Ω(err).ShouldNot(HaveOccurred())

			err = database.SaveBuildEndTime(build.ID, endTime)
			Ω(err).ShouldNot(HaveOccurred())

			build, err = database.GetBuild(build.ID)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(build.StartTime.Unix()).Should(Equal(startTime.Unix()))
			Ω(build.EndTime.Unix()).Should(Equal(endTime.Unix()))
		})

		It("can report a job's latest running and finished builds", func() {
			finished, next, err := database.GetJobFinishedAndNextBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(next).Should(BeNil())
			Ω(finished).Should(BeNil())

			finishedBuild, err := database.CreateJobBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			err = database.SaveBuildStatus(finishedBuild.ID, db.StatusSucceeded)
			Ω(err).ShouldNot(HaveOccurred())

			finished, next, err = database.GetJobFinishedAndNextBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(next).Should(BeNil())
			Ω(finished.ID).Should(Equal(finishedBuild.ID))

			nextBuild, err := database.CreateJobBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			started, err := database.StartBuild(nextBuild.ID, "some-engine", "meta")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(started).Should(BeTrue())

			finished, next, err = database.GetJobFinishedAndNextBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(next.ID).Should(Equal(nextBuild.ID))
			Ω(finished.ID).Should(Equal(finishedBuild.ID))

			anotherRunningBuild, err := database.CreateJobBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			finished, next, err = database.GetJobFinishedAndNextBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(next.ID).Should(Equal(nextBuild.ID)) // not anotherRunningBuild
			Ω(finished.ID).Should(Equal(finishedBuild.ID))

			started, err = database.StartBuild(anotherRunningBuild.ID, "some-engine", "meta")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(started).Should(BeTrue())

			finished, next, err = database.GetJobFinishedAndNextBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(next.ID).Should(Equal(nextBuild.ID)) // not anotherRunningBuild
			Ω(finished.ID).Should(Equal(finishedBuild.ID))

			err = database.SaveBuildStatus(nextBuild.ID, db.StatusSucceeded)
			Ω(err).ShouldNot(HaveOccurred())

			finished, next, err = database.GetJobFinishedAndNextBuild("some-job")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(next.ID).Should(Equal(anotherRunningBuild.ID))
			Ω(finished.ID).Should(Equal(nextBuild.ID))
		})

		Describe("locking", func() {
			It("can be done generically with a unique name", func() {
				lock, err := database.AcquireWriteLock([]db.NamedLock{db.ResourceLock("a-name")})
				Ω(err).ShouldNot(HaveOccurred())

				secondLockCh := make(chan db.Lock, 1)

				go func() {
					defer GinkgoRecover()

					secondLock, err := database.AcquireWriteLock([]db.NamedLock{db.ResourceLock("a-name")})
					Ω(err).ShouldNot(HaveOccurred())

					secondLockCh <- secondLock
				}()

				Consistently(secondLockCh).ShouldNot(Receive())

				err = lock.Release()
				Ω(err).ShouldNot(HaveOccurred())

				var secondLock db.Lock
				Eventually(secondLockCh).Should(Receive(&secondLock))

				err = secondLock.Release()
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("can be done without waiting", func() {
				lock, err := database.AcquireWriteLockImmediately([]db.NamedLock{db.ResourceLock("a-name")})
				Ω(err).ShouldNot(HaveOccurred())

				secondLock, err := database.AcquireWriteLockImmediately([]db.NamedLock{db.ResourceLock("a-name")})
				Ω(err).Should(HaveOccurred())
				Ω(secondLock).Should(BeNil())

				err = lock.Release()
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("does not let anyone write if someone is reading", func() {
				lock, err := database.AcquireReadLock([]db.NamedLock{db.ResourceLock("a-name")})
				Ω(err).ShouldNot(HaveOccurred())

				secondLockCh := make(chan db.Lock, 1)

				go func() {
					defer GinkgoRecover()

					secondLock, err := database.AcquireWriteLock([]db.NamedLock{db.ResourceLock("a-name")})
					Ω(err).ShouldNot(HaveOccurred())

					secondLockCh <- secondLock
				}()

				Consistently(secondLockCh).ShouldNot(Receive())

				err = lock.Release()
				Ω(err).ShouldNot(HaveOccurred())

				var secondLock db.Lock
				Eventually(secondLockCh).Should(Receive(&secondLock))

				err = secondLock.Release()
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("does not let anyone read if someone is writing", func() {
				lock, err := database.AcquireWriteLock([]db.NamedLock{db.ResourceLock("a-name")})
				Ω(err).ShouldNot(HaveOccurred())

				secondLockCh := make(chan db.Lock, 1)

				go func() {
					defer GinkgoRecover()

					secondLock, err := database.AcquireReadLock([]db.NamedLock{db.ResourceLock("a-name")})
					Ω(err).ShouldNot(HaveOccurred())

					secondLockCh <- secondLock
				}()

				Consistently(secondLockCh).ShouldNot(Receive())

				err = lock.Release()
				Ω(err).ShouldNot(HaveOccurred())

				var secondLock db.Lock
				Eventually(secondLockCh).Should(Receive(&secondLock))

				err = secondLock.Release()
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("lets many reads simultaneously", func() {
				lock, err := database.AcquireReadLock([]db.NamedLock{db.ResourceLock("a-name")})
				Ω(err).ShouldNot(HaveOccurred())

				secondLockCh := make(chan db.Lock, 1)

				go func() {
					defer GinkgoRecover()

					secondLock, err := database.AcquireReadLock([]db.NamedLock{db.ResourceLock("a-name")})
					Ω(err).ShouldNot(HaveOccurred())

					secondLockCh <- secondLock
				}()

				var secondLock db.Lock
				Eventually(secondLockCh).Should(Receive(&secondLock))

				err = secondLock.Release()
				Ω(err).ShouldNot(HaveOccurred())

				err = lock.Release()
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("can be done multiple times if using different locks", func() {
				lock, err := database.AcquireWriteLock([]db.NamedLock{db.ResourceLock("name-1")})
				Ω(err).ShouldNot(HaveOccurred())

				var secondLock db.Lock
				secondLockCh := make(chan db.Lock, 1)

				go func() {
					defer GinkgoRecover()

					secondLock, err := database.AcquireWriteLock([]db.NamedLock{db.ResourceLock("name-2")})
					Ω(err).ShouldNot(HaveOccurred())

					secondLockCh <- secondLock
				}()

				Eventually(secondLockCh).Should(Receive(&secondLock))

				err = lock.Release()
				Ω(err).ShouldNot(HaveOccurred())

				err = secondLock.Release()
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("can be done for multiple locks at a time", func() {
				lock, err := database.AcquireWriteLock([]db.NamedLock{db.ResourceLock("name-1"), db.ResourceLock("name-2")})
				Ω(err).ShouldNot(HaveOccurred())

				secondLockCh := make(chan db.Lock, 1)

				go func() {
					defer GinkgoRecover()

					secondLock, err := database.AcquireWriteLock([]db.NamedLock{db.ResourceLock("name-1")})
					Ω(err).ShouldNot(HaveOccurred())

					secondLockCh <- secondLock
				}()

				Consistently(secondLockCh).ShouldNot(Receive())

				thirdLockCh := make(chan db.Lock, 1)

				go func() {
					defer GinkgoRecover()

					thirdLock, err := database.AcquireWriteLock([]db.NamedLock{db.ResourceLock("name-2")})
					Ω(err).ShouldNot(HaveOccurred())

					thirdLockCh <- thirdLock
				}()

				Consistently(thirdLockCh).ShouldNot(Receive())

				err = lock.Release()
				Ω(err).ShouldNot(HaveOccurred())

				var secondLock db.Lock
				Eventually(secondLockCh).Should(Receive(&secondLock))

				var thirdLock db.Lock
				Eventually(thirdLockCh).Should(Receive(&thirdLock))

				err = secondLock.Release()
				Ω(err).ShouldNot(HaveOccurred())

				err = thirdLock.Release()
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("cleans up after releasing", func() {
				lock, err := database.AcquireWriteLock([]db.NamedLock{db.ResourceLock("a-name")})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(database.ListLocks()).Should(ContainElement(db.ResourceLock("a-name").Name()))

				secondLockCh := make(chan db.Lock, 1)

				go func() {
					defer GinkgoRecover()

					secondLock, err := database.AcquireWriteLock([]db.NamedLock{db.ResourceLock("a-name")})
					Ω(err).ShouldNot(HaveOccurred())

					secondLockCh <- secondLock
				}()

				Consistently(secondLockCh).ShouldNot(Receive())

				err = lock.Release()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(database.ListLocks()).Should(ContainElement(db.ResourceLock("a-name").Name()))

				var secondLock db.Lock
				Eventually(secondLockCh).Should(Receive(&secondLock))

				err = secondLock.Release()
				Ω(err).ShouldNot(HaveOccurred())

				Ω(database.ListLocks()).Should(BeEmpty())
			})
		})

		Describe("saving build inputs", func() {
			buildMetadata := []db.MetadataField{
				{
					Name:  "meta1",
					Value: "value1",
				},
				{
					Name:  "meta2",
					Value: "value2",
				},
			}

			vr1 := db.VersionedResource{
				Resource: "some-resource",
				Type:     "some-type",
				Source:   db.Source{"some": "source"},
				Version:  db.Version{"ver": "1"},
				Metadata: buildMetadata,
			}

			vr2 := db.VersionedResource{
				Resource: "some-other-resource",
				Type:     "some-type",
				Source:   db.Source{"some": "other-source"},
				Version:  db.Version{"ver": "2"},
			}

			It("saves build's inputs and outputs as versioned resources", func() {
				build, err := database.CreateJobBuild("some-job")
				Ω(err).ShouldNot(HaveOccurred())

				input1 := db.BuildInput{
					Name:              "some-input",
					VersionedResource: vr1,
				}

				input2 := db.BuildInput{
					Name:              "some-other-input",
					VersionedResource: vr2,
				}

				otherInput := db.BuildInput{
					Name:              "some-random-input",
					VersionedResource: vr2,
				}

				err = database.SaveBuildInput(build.ID, input1)
				Ω(err).ShouldNot(HaveOccurred())

				_, err = database.GetJobBuildForInputs("some-job", []db.BuildInput{
					input1,
					input2,
				})
				Ω(err).Should(HaveOccurred())

				err = database.SaveBuildInput(build.ID, otherInput)
				Ω(err).ShouldNot(HaveOccurred())

				_, err = database.GetJobBuildForInputs("some-job", []db.BuildInput{
					input1,
					input2,
				})
				Ω(err).Should(HaveOccurred())

				err = database.SaveBuildInput(build.ID, input2)
				Ω(err).ShouldNot(HaveOccurred())

				foundBuild, err := database.GetJobBuildForInputs("some-job", []db.BuildInput{
					input1,
					input2,
				})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(foundBuild).Should(Equal(build))

				err = database.SaveBuildOutput(build.ID, vr1)
				Ω(err).ShouldNot(HaveOccurred())

				modifiedVR2 := vr2
				modifiedVR2.Version = db.Version{"ver": "3"}

				err = database.SaveBuildOutput(build.ID, modifiedVR2)
				Ω(err).ShouldNot(HaveOccurred())

				err = database.SaveBuildOutput(build.ID, vr2)
				Ω(err).ShouldNot(HaveOccurred())

				inputs, outputs, err := database.GetBuildResources(build.ID)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(inputs).Should(ConsistOf([]db.BuildInput{
					{Name: "some-input", VersionedResource: vr1, FirstOccurrence: true},
					{Name: "some-other-input", VersionedResource: vr2, FirstOccurrence: true},
					{Name: "some-random-input", VersionedResource: vr2, FirstOccurrence: true},
				}))
				Ω(outputs).Should(ConsistOf([]db.BuildOutput{
					{VersionedResource: modifiedVR2},
				}))

				duplicateBuild, err := database.CreateJobBuild("some-job")
				Ω(err).ShouldNot(HaveOccurred())

				err = database.SaveBuildInput(duplicateBuild.ID, db.BuildInput{
					Name:              "other-build-input",
					VersionedResource: vr1,
				})
				Ω(err).ShouldNot(HaveOccurred())

				err = database.SaveBuildInput(duplicateBuild.ID, db.BuildInput{
					Name:              "other-build-other-input",
					VersionedResource: vr2,
				})
				Ω(err).ShouldNot(HaveOccurred())

				inputs, _, err = database.GetBuildResources(duplicateBuild.ID)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(inputs).Should(ConsistOf([]db.BuildInput{
					{Name: "other-build-input", VersionedResource: vr1, FirstOccurrence: false},
					{Name: "other-build-other-input", VersionedResource: vr2, FirstOccurrence: false},
				}))

				newBuildInOtherJob, err := database.CreateJobBuild("some-other-job")
				Ω(err).ShouldNot(HaveOccurred())

				err = database.SaveBuildInput(newBuildInOtherJob.ID, db.BuildInput{
					Name:              "other-job-input",
					VersionedResource: vr1,
				})
				Ω(err).ShouldNot(HaveOccurred())

				err = database.SaveBuildInput(newBuildInOtherJob.ID, db.BuildInput{
					Name:              "other-job-other-input",
					VersionedResource: vr2,
				})
				Ω(err).ShouldNot(HaveOccurred())

				inputs, _, err = database.GetBuildResources(newBuildInOtherJob.ID)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(inputs).Should(ConsistOf([]db.BuildInput{
					{Name: "other-job-input", VersionedResource: vr1, FirstOccurrence: true},
					{Name: "other-job-other-input", VersionedResource: vr2, FirstOccurrence: true},
				}))
			})

			It("updates metadata of existing inputs resources", func() {
				build, err := database.CreateJobBuild("some-job")
				Ω(err).ShouldNot(HaveOccurred())

				err = database.SaveBuildInput(build.ID, db.BuildInput{
					Name:              "some-input",
					VersionedResource: vr2,
				})
				Ω(err).ShouldNot(HaveOccurred())

				inputs, _, err := database.GetBuildResources(build.ID)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(inputs).Should(ConsistOf([]db.BuildInput{
					{Name: "some-input", VersionedResource: vr2, FirstOccurrence: true},
				}))

				withMetadata := vr2
				withMetadata.Metadata = buildMetadata

				err = database.SaveBuildInput(build.ID, db.BuildInput{
					Name:              "some-other-input",
					VersionedResource: withMetadata,
				})
				Ω(err).ShouldNot(HaveOccurred())

				inputs, _, err = database.GetBuildResources(build.ID)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(inputs).Should(ConsistOf([]db.BuildInput{
					{Name: "some-input", VersionedResource: withMetadata, FirstOccurrence: true},
					{Name: "some-other-input", VersionedResource: withMetadata, FirstOccurrence: true},
				}))

				err = database.SaveBuildInput(build.ID, db.BuildInput{
					Name:              "some-input",
					VersionedResource: withMetadata,
				})
				Ω(err).ShouldNot(HaveOccurred())

				inputs, _, err = database.GetBuildResources(build.ID)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(inputs).Should(ConsistOf([]db.BuildInput{
					{Name: "some-input", VersionedResource: withMetadata, FirstOccurrence: true},
					{Name: "some-other-input", VersionedResource: withMetadata, FirstOccurrence: true},
				}))
			})

			It("can be done on build creation", func() {
				inputs := []db.BuildInput{
					{Name: "first-input", VersionedResource: vr1},
					{Name: "second-input", VersionedResource: vr2},
				}

				pending, err := database.CreateJobBuildWithInputs("some-job", inputs)
				Ω(err).ShouldNot(HaveOccurred())

				foundBuild, err := database.GetJobBuildForInputs("some-job", inputs)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(foundBuild).Should(Equal(pending))

				nextPending, pendingInputs, err := database.GetNextPendingBuild("some-job")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(nextPending).Should(Equal(pending))
				Ω(pendingInputs).Should(ConsistOf([]db.BuildInput{
					{Name: "first-input", VersionedResource: vr1, FirstOccurrence: true},
					{Name: "second-input", VersionedResource: vr2, FirstOccurrence: true},
				}))
			})
		})

		Describe("saving versioned resources", func() {
			It("updates the latest versioned resource", func() {
				vr1 := db.VersionedResource{
					Resource: "some-resource",
					Type:     "some-type",
					Source:   db.Source{"some": "source"},
					Version:  db.Version{"version": "1"},
					Metadata: []db.MetadataField{
						{Name: "meta1", Value: "value1"},
					},
				}

				vr2 := db.VersionedResource{
					Resource: "some-resource",
					Type:     "some-type",
					Source:   db.Source{"some": "source"},
					Version:  db.Version{"version": "2"},
					Metadata: []db.MetadataField{
						{Name: "meta2", Value: "value2"},
					},
				}

				vr3 := db.VersionedResource{
					Resource: "some-resource",
					Type:     "some-type",
					Source:   db.Source{"some": "source"},
					Version:  db.Version{"version": "3"},
					Metadata: []db.MetadataField{
						{Name: "meta3", Value: "value3"},
					},
				}

				err := database.SaveVersionedResource(vr1)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(database.GetLatestVersionedResource("some-resource")).Should(Equal(vr1))

				err = database.SaveVersionedResource(vr2)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(database.GetLatestVersionedResource("some-resource")).Should(Equal(vr2))

				err = database.SaveVersionedResource(vr3)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(database.GetLatestVersionedResource("some-resource")).Should(Equal(vr3))
			})

			It("overwrites the existing source and metadata for the same version", func() {
				vr := db.VersionedResource{
					Resource: "some-resource",
					Type:     "some-type",
					Source:   db.Source{"some": "source"},
					Version:  db.Version{"version": "1"},
					Metadata: []db.MetadataField{
						{Name: "meta1", Value: "value1"},
					},
				}

				err := database.SaveVersionedResource(vr)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(database.GetLatestVersionedResource("some-resource")).Should(Equal(vr))

				modified := vr
				modified.Source["additional"] = "data"
				modified.Metadata[0].Value = "modified-value1"

				err = database.SaveVersionedResource(modified)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(database.GetLatestVersionedResource("some-resource")).Should(Equal(modified))
			})
		})

		Describe("determining the inputs for a job", func() {
			It("ensures that versions from jobs mentioned in two input's 'passed' sections came from the same builds", func() {
				j1b1, err := database.CreateJobBuild("job-1")
				Ω(err).ShouldNot(HaveOccurred())

				j2b1, err := database.CreateJobBuild("job-2")
				Ω(err).ShouldNot(HaveOccurred())

				sb1, err := database.CreateJobBuild("shared-job")
				Ω(err).ShouldNot(HaveOccurred())

				err = database.SaveBuildOutput(sb1.ID, db.VersionedResource{
					Resource: "resource-1",
					Type:     "some-type",
					Version:  db.Version{"v": "r1-common-to-shared-and-j1"},
				})
				Ω(err).ShouldNot(HaveOccurred())

				err = database.SaveBuildOutput(sb1.ID, db.VersionedResource{
					Resource: "resource-2",
					Type:     "some-type",
					Version:  db.Version{"v": "r2-common-to-shared-and-j2"},
				})
				Ω(err).ShouldNot(HaveOccurred())

				err = database.SaveBuildOutput(j1b1.ID, db.VersionedResource{
					Resource: "resource-1",
					Type:     "some-type",
					Version:  db.Version{"v": "r1-common-to-shared-and-j1"},
				})
				Ω(err).ShouldNot(HaveOccurred())

				err = database.SaveBuildOutput(j2b1.ID, db.VersionedResource{
					Resource: "resource-2",
					Type:     "some-type",
					Version:  db.Version{"v": "r2-common-to-shared-and-j2"},
				})
				Ω(err).ShouldNot(HaveOccurred())

				Ω(database.GetLatestInputVersions([]atc.InputConfig{
					{
						Resource: "resource-1",
						Passed:   []string{"shared-job", "job-1"},
					},
					{
						Resource: "resource-2",
						Passed:   []string{"shared-job", "job-2"},
					},
				})).Should(Equal(db.VersionedResources{
					{
						Resource: "resource-1",
						Type:     "some-type",
						Version:  db.Version{"v": "r1-common-to-shared-and-j1"},
					},
					{
						Resource: "resource-2",
						Type:     "some-type",
						Version:  db.Version{"v": "r2-common-to-shared-and-j2"},
					},
				}))

				sb2, err := database.CreateJobBuild("shared-job")
				Ω(err).ShouldNot(HaveOccurred())

				j1b2, err := database.CreateJobBuild("job-1")
				Ω(err).ShouldNot(HaveOccurred())

				j2b2, err := database.CreateJobBuild("job-2")
				Ω(err).ShouldNot(HaveOccurred())

				err = database.SaveBuildOutput(sb2.ID, db.VersionedResource{
					Resource: "resource-1",
					Type:     "some-type",
					Version:  db.Version{"v": "new-r1-common-to-shared-and-j1"},
				})
				Ω(err).ShouldNot(HaveOccurred())

				err = database.SaveBuildOutput(sb2.ID, db.VersionedResource{
					Resource: "resource-2",
					Type:     "some-type",
					Version:  db.Version{"v": "new-r2-common-to-shared-and-j2"},
				})
				Ω(err).ShouldNot(HaveOccurred())

				err = database.SaveBuildOutput(j1b2.ID, db.VersionedResource{
					Resource: "resource-1",
					Type:     "some-type",
					Version:  db.Version{"v": "new-r1-common-to-shared-and-j1"},
				})
				Ω(err).ShouldNot(HaveOccurred())

				// do NOT save resource-2 as an output of job-2

				Ω(database.GetLatestInputVersions([]atc.InputConfig{
					{
						Resource: "resource-1",
						Passed:   []string{"shared-job", "job-1"},
					},
					{
						Resource: "resource-2",
						Passed:   []string{"shared-job", "job-2"},
					},
				})).Should(Equal(db.VersionedResources{
					{
						Resource: "resource-1",
						Type:     "some-type",
						Version:  db.Version{"v": "r1-common-to-shared-and-j1"},
					},
					{
						Resource: "resource-2",
						Type:     "some-type",
						Version:  db.Version{"v": "r2-common-to-shared-and-j2"},
					},
				}))

				// now save the output of resource-2 job-2
				err = database.SaveBuildOutput(j2b2.ID, db.VersionedResource{
					Resource: "resource-2",
					Type:     "some-type",
					Version:  db.Version{"v": "new-r2-common-to-shared-and-j2"},
				})
				Ω(err).ShouldNot(HaveOccurred())

				Ω(database.GetLatestInputVersions([]atc.InputConfig{
					{
						Resource: "resource-1",
						Passed:   []string{"shared-job", "job-1"},
					},
					{
						Resource: "resource-2",
						Passed:   []string{"shared-job", "job-2"},
					},
				})).Should(Equal(db.VersionedResources{
					{
						Resource: "resource-1",
						Type:     "some-type",
						Version:  db.Version{"v": "new-r1-common-to-shared-and-j1"},
					},
					{
						Resource: "resource-2",
						Type:     "some-type",
						Version:  db.Version{"v": "new-r2-common-to-shared-and-j2"},
					},
				}))

				// save newer versions; should be new latest
				for i := 0; i < 10; i++ {
					version := fmt.Sprintf("version-%d", i+1)

					err = database.SaveBuildOutput(sb1.ID, db.VersionedResource{
						Resource: "resource-1",
						Type:     "some-type",
						Version:  db.Version{"v": version + "-r1-common-to-shared-and-j1"},
					})
					Ω(err).ShouldNot(HaveOccurred())

					err = database.SaveBuildOutput(sb1.ID, db.VersionedResource{
						Resource: "resource-2",
						Type:     "some-type",
						Version:  db.Version{"v": version + "-r2-common-to-shared-and-j2"},
					})
					Ω(err).ShouldNot(HaveOccurred())

					err = database.SaveBuildOutput(j1b1.ID, db.VersionedResource{
						Resource: "resource-1",
						Type:     "some-type",
						Version:  db.Version{"v": version + "-r1-common-to-shared-and-j1"},
					})
					Ω(err).ShouldNot(HaveOccurred())

					err = database.SaveBuildOutput(j2b1.ID, db.VersionedResource{
						Resource: "resource-2",
						Type:     "some-type",
						Version:  db.Version{"v": version + "-r2-common-to-shared-and-j2"},
					})
					Ω(err).ShouldNot(HaveOccurred())

					Ω(database.GetLatestInputVersions([]atc.InputConfig{
						{
							Resource: "resource-1",
							Passed:   []string{"shared-job", "job-1"},
						},
						{
							Resource: "resource-2",
							Passed:   []string{"shared-job", "job-2"},
						},
					})).Should(Equal(db.VersionedResources{
						{
							Resource: "resource-1",
							Type:     "some-type",
							Version:  db.Version{"v": version + "-r1-common-to-shared-and-j1"},
						},
						{
							Resource: "resource-2",
							Type:     "some-type",
							Version:  db.Version{"v": version + "-r2-common-to-shared-and-j2"},
						},
					}))
				}
			})
		})

		Context("when the first build is created", func() {
			var firstBuild db.Build

			var job string

			BeforeEach(func() {
				var err error

				job = "some-job"

				firstBuild, err = database.CreateJobBuild(job)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(firstBuild.Name).Should(Equal("1"))
				Ω(firstBuild.Status).Should(Equal(db.StatusPending))
			})

			Context("and then aborted", func() {
				BeforeEach(func() {
					err := database.SaveBuildStatus(firstBuild.ID, db.StatusAborted)
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("changes the state to aborted", func() {
					build, err := database.GetJobBuild(job, firstBuild.Name)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(build.Status).Should(Equal(db.StatusAborted))
				})

				Describe("scheduling the build", func() {
					It("fails", func() {
						scheduled, err := database.ScheduleBuild(firstBuild.ID, false)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeFalse())
					})
				})
			})

			Context("and then scheduled", func() {
				BeforeEach(func() {
					scheduled, err := database.ScheduleBuild(firstBuild.ID, false)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeTrue())
				})

				Context("and then aborted", func() {
					BeforeEach(func() {
						err := database.SaveBuildStatus(firstBuild.ID, db.StatusAborted)
						Ω(err).ShouldNot(HaveOccurred())
					})

					It("changes the state to aborted", func() {
						build, err := database.GetJobBuild(job, firstBuild.Name)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(build.Status).Should(Equal(db.StatusAborted))
					})

					Describe("starting the build", func() {
						It("fails", func() {
							started, err := database.StartBuild(firstBuild.ID, "some-engine", "some-meta")
							Ω(err).ShouldNot(HaveOccurred())
							Ω(started).Should(BeFalse())
						})
					})
				})
			})

			Describe("scheduling the build", func() {
				It("succeeds", func() {
					scheduled, err := database.ScheduleBuild(firstBuild.ID, false)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(scheduled).Should(BeTrue())
				})

				Context("serially", func() {
					It("succeeds", func() {
						scheduled, err := database.ScheduleBuild(firstBuild.ID, true)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(scheduled).Should(BeTrue())
					})
				})
			})

			Context("and a second build is created", func() {
				var secondBuild db.Build

				Context("for a different job", func() {
					BeforeEach(func() {
						var err error

						secondBuild, err = database.CreateJobBuild("some-other-job")
						Ω(err).ShouldNot(HaveOccurred())
						Ω(secondBuild.Name).Should(Equal("1"))
						Ω(secondBuild.Status).Should(Equal(db.StatusPending))
					})

					Describe("scheduling the second build", func() {
						It("succeeds", func() {
							scheduled, err := database.ScheduleBuild(secondBuild.ID, false)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(scheduled).Should(BeTrue())
						})

						Describe("serially", func() {
							It("succeeds", func() {
								scheduled, err := database.ScheduleBuild(secondBuild.ID, true)
								Ω(err).ShouldNot(HaveOccurred())
								Ω(scheduled).Should(BeTrue())
							})
						})
					})
				})

				Context("for the same job", func() {
					BeforeEach(func() {
						var err error

						secondBuild, err = database.CreateJobBuild(job)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(secondBuild.Name).Should(Equal("2"))
						Ω(secondBuild.Status).Should(Equal(db.StatusPending))
					})

					Describe("scheduling the second build", func() {
						It("succeeds", func() {
							scheduled, err := database.ScheduleBuild(secondBuild.ID, false)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(scheduled).Should(BeTrue())
						})

						Describe("serially", func() {
							It("fails", func() {
								scheduled, err := database.ScheduleBuild(secondBuild.ID, true)
								Ω(err).ShouldNot(HaveOccurred())
								Ω(scheduled).Should(BeFalse())
							})
						})
					})

					Describe("after the first build schedules", func() {
						BeforeEach(func() {
							scheduled, err := database.ScheduleBuild(firstBuild.ID, false)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(scheduled).Should(BeTrue())
						})

						Context("when the second build is scheduled serially", func() {
							It("fails", func() {
								scheduled, err := database.ScheduleBuild(secondBuild.ID, true)
								Ω(err).ShouldNot(HaveOccurred())
								Ω(scheduled).Should(BeFalse())
							})
						})

						for _, s := range []db.Status{db.StatusSucceeded, db.StatusFailed, db.StatusErrored} {
							status := s

							Context("and the first build's status changes to "+string(status), func() {
								BeforeEach(func() {
									err := database.SaveBuildStatus(firstBuild.ID, status)
									Ω(err).ShouldNot(HaveOccurred())
								})

								Context("and the second build is scheduled serially", func() {
									It("succeeds", func() {
										scheduled, err := database.ScheduleBuild(secondBuild.ID, true)
										Ω(err).ShouldNot(HaveOccurred())
										Ω(scheduled).Should(BeTrue())
									})
								})
							})
						}
					})

					Describe("after the first build is aborted", func() {
						BeforeEach(func() {
							err := database.SaveBuildStatus(firstBuild.ID, db.StatusAborted)
							Ω(err).ShouldNot(HaveOccurred())
						})

						Context("when the second build is scheduled serially", func() {
							It("succeeds", func() {
								scheduled, err := database.ScheduleBuild(secondBuild.ID, true)
								Ω(err).ShouldNot(HaveOccurred())
								Ω(scheduled).Should(BeTrue())
							})
						})
					})

					Context("and a third build is created", func() {
						var thirdBuild db.Build

						BeforeEach(func() {
							var err error

							thirdBuild, err = database.CreateJobBuild(job)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(thirdBuild.Name).Should(Equal("3"))
							Ω(thirdBuild.Status).Should(Equal(db.StatusPending))
						})

						Context("and the first build finishes", func() {
							BeforeEach(func() {
								err := database.SaveBuildStatus(firstBuild.ID, db.StatusSucceeded)
								Ω(err).ShouldNot(HaveOccurred())
							})

							Context("and the third build is scheduled serially", func() {
								It("fails, as it would have jumped the queue", func() {
									scheduled, err := database.ScheduleBuild(thirdBuild.ID, true)
									Ω(err).ShouldNot(HaveOccurred())
									Ω(scheduled).Should(BeFalse())
								})
							})
						})

						Context("and then scheduled", func() {
							It("succeeds", func() {
								scheduled, err := database.ScheduleBuild(thirdBuild.ID, false)
								Ω(err).ShouldNot(HaveOccurred())
								Ω(scheduled).Should(BeTrue())
							})

							Describe("serially", func() {
								It("fails", func() {
									scheduled, err := database.ScheduleBuild(thirdBuild.ID, true)
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
}
