package db_test

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type dbSharedBehaviorInput struct {
	db.DB
	PipelineDB db.PipelineDB
}

func dbSharedBehavior(database *dbSharedBehaviorInput) func() {
	return func() {
		Describe("CreatePipe", func() {
			It("saves a pipe to the db", func() {
				myGuid, err := uuid.NewV4()
				Ω(err).ShouldNot(HaveOccurred())

				err = database.CreatePipe(myGuid.String(), "a-url")
				Ω(err).ShouldNot(HaveOccurred())

				pipe, err := database.GetPipe(myGuid.String())
				Ω(err).ShouldNot(HaveOccurred())
				Ω(pipe.ID).Should(Equal(myGuid.String()))
				Ω(pipe.URL).Should(Equal("a-url"))
			})
		})

		It("saves and propagates events correctly", func() {
			build, err := database.CreateOneOffBuild()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(build.Name).Should(Equal("1"))

			By("allowing you to subscribe when no events have yet occurred")
			events, err := database.GetBuildEvents(build.ID, 0)
			Ω(err).ShouldNot(HaveOccurred())

			defer events.Close()

			By("saving them in order")
			err = database.SaveBuildEvent(build.ID, event.Log{
				Payload: "some ",
			})
			Ω(err).ShouldNot(HaveOccurred())

			Ω(events.Next()).Should(Equal(event.Log{
				Payload: "some ",
			}))

			err = database.SaveBuildEvent(build.ID, event.Log{
				Payload: "log",
			})
			Ω(err).ShouldNot(HaveOccurred())

			Ω(events.Next()).Should(Equal(event.Log{
				Payload: "log",
			}))

			By("allowing you to subscribe from an offset")
			eventsFrom1, err := database.GetBuildEvents(build.ID, 1)
			Ω(err).ShouldNot(HaveOccurred())

			defer eventsFrom1.Close()

			Ω(eventsFrom1.Next()).Should(Equal(event.Log{
				Payload: "log",
			}))

			By("notifying those waiting on events as soon as they're saved")
			nextEvent := make(chan atc.Event)
			nextErr := make(chan error)

			go func() {
				event, err := events.Next()
				if err != nil {
					nextErr <- err
				} else {
					nextEvent <- event
				}
			}()

			Consistently(nextEvent).ShouldNot(Receive())
			Consistently(nextErr).ShouldNot(Receive())

			err = database.SaveBuildEvent(build.ID, event.Log{
				Payload: "log 2",
			})
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(nextEvent).Should(Receive(Equal(event.Log{
				Payload: "log 2",
			})))

			By("returning ErrBuildEventStreamClosed for Next calls after Close")
			events3, err := database.GetBuildEvents(build.ID, 0)
			Ω(err).ShouldNot(HaveOccurred())

			events3.Close()

			_, err = events3.Next()
			Ω(err).Should(Equal(db.ErrBuildEventStreamClosed))
		})

		It("saves and emits status events", func() {
			build, err := database.CreateOneOffBuild()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(build.Name).Should(Equal("1"))

			By("allowing you to subscribe when no events have yet occurred")
			events, err := database.GetBuildEvents(build.ID, 0)
			Ω(err).ShouldNot(HaveOccurred())

			defer events.Close()

			By("emitting a status event when started")
			started, err := database.StartBuild(build.ID, "engine", "metadata")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(started).Should(BeTrue())

			startedBuild, err := database.GetBuild(build.ID)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(events.Next()).Should(Equal(event.Status{
				Status: atc.StatusStarted,
				Time:   startedBuild.StartTime.Unix(),
			}))

			By("emitting a status event when finished")
			err = database.FinishBuild(build.ID, db.StatusSucceeded)
			Ω(err).ShouldNot(HaveOccurred())

			finishedBuild, err := database.GetBuild(build.ID)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(events.Next()).Should(Equal(event.Status{
				Status: atc.StatusSucceeded,
				Time:   finishedBuild.EndTime.Unix(),
			}))

			By("ending the stream when finished")
			_, err = events.Next()
			Ω(err).Should(Equal(db.ErrEndOfBuildEventStream))
		})

		It("can keep track of workers", func() {
			Ω(database.Workers()).Should(BeEmpty())

			infoA := db.WorkerInfo{
				Addr:             "1.2.3.4:7777",
				ActiveContainers: 42,
				ResourceTypes: []atc.WorkerResourceType{
					{Type: "some-resource-a", Image: "some-image-a"},
				},
				Platform: "webos",
				Tags:     []string{"palm", "was", "great"},
			}

			infoB := db.WorkerInfo{
				Addr:             "1.2.3.4:8888",
				ActiveContainers: 42,
				ResourceTypes: []atc.WorkerResourceType{
					{Type: "some-resource-b", Image: "some-image-b"},
				},
				Platform: "plan9",
				Tags:     []string{"russ", "cox", "was", "here"},
			}

			By("persisting workers with no TTLs")
			err := database.SaveWorker(infoA, 0)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(database.Workers()).Should(ConsistOf(infoA))

			By("being idempotent")
			err = database.SaveWorker(infoA, 0)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(database.Workers()).Should(ConsistOf(infoA))

			By("expiring TTLs")
			ttl := 1 * time.Second

			err = database.SaveWorker(infoB, ttl)
			Ω(err).ShouldNot(HaveOccurred())

			Consistently(database.Workers, ttl/2).Should(ConsistOf(infoA, infoB))
			Eventually(database.Workers, 2*ttl).Should(ConsistOf(infoA))

			By("overwriting TTLs")
			err = database.SaveWorker(infoA, ttl)
			Ω(err).ShouldNot(HaveOccurred())

			Consistently(database.Workers, ttl/2).Should(ConsistOf(infoA))
			Eventually(database.Workers, 2*ttl).Should(BeEmpty())
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

			jobBuild, err := database.PipelineDB.CreateJobBuild("some-other-job")
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

		Describe("GetAllStartedBuilds", func() {
			var build1 db.Build
			var build2 db.Build
			var build3 db.Build
			BeforeEach(func() {
				var err error

				build1, err = database.CreateOneOffBuild()
				Ω(err).ShouldNot(HaveOccurred())

				build2, err = database.PipelineDB.CreateJobBuild("some-job")
				Ω(err).ShouldNot(HaveOccurred())

				build3, err = database.CreateOneOffBuild()
				Ω(err).ShouldNot(HaveOccurred())

				started, err := database.StartBuild(build1.ID, "some-engine", "so-meta")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(started).Should(BeTrue())

				started, err = database.StartBuild(build2.ID, "some-engine", "so-meta")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(started).Should(BeTrue())
			})

			It("returns all builds that have been started, regardless of pipeline", func() {
				builds, err := database.GetAllStartedBuilds()
				Ω(err).ShouldNot(HaveOccurred())

				Ω(len(builds)).Should(Equal(2))

				build1, err := database.GetBuild(build1.ID)
				Ω(err).ShouldNot(HaveOccurred())
				build2, err := database.GetBuild(build2.ID)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(builds).Should(ConsistOf(build1, build2))
			})
		})

		Describe("locking", func() {
			It("can be done generically with a unique name", func() {
				lock, err := database.AcquireWriteLock([]db.NamedLock{db.ResourceCheckingLock("a-name")})
				Ω(err).ShouldNot(HaveOccurred())

				secondLockCh := make(chan db.Lock, 1)

				go func() {
					defer GinkgoRecover()

					secondLock, err := database.AcquireWriteLock([]db.NamedLock{db.ResourceCheckingLock("a-name")})
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
				lock, err := database.AcquireWriteLockImmediately([]db.NamedLock{db.ResourceCheckingLock("a-name")})
				Ω(err).ShouldNot(HaveOccurred())

				secondLock, err := database.AcquireWriteLockImmediately([]db.NamedLock{db.ResourceCheckingLock("a-name")})
				Ω(err).Should(Equal(db.ErrLockNotAvailable))
				Ω(secondLock).Should(BeNil())

				err = lock.Release()
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("does not let anyone write if someone is reading", func() {
				lock, err := database.AcquireReadLock([]db.NamedLock{db.ResourceCheckingLock("a-name")})
				Ω(err).ShouldNot(HaveOccurred())

				secondLockCh := make(chan db.Lock, 1)

				go func() {
					defer GinkgoRecover()

					secondLock, err := database.AcquireWriteLock([]db.NamedLock{db.ResourceCheckingLock("a-name")})
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
				lock, err := database.AcquireWriteLock([]db.NamedLock{db.ResourceCheckingLock("a-name")})
				Ω(err).ShouldNot(HaveOccurred())

				secondLockCh := make(chan db.Lock, 1)

				go func() {
					defer GinkgoRecover()

					secondLock, err := database.AcquireReadLock([]db.NamedLock{db.ResourceCheckingLock("a-name")})
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
				lock, err := database.AcquireReadLock([]db.NamedLock{db.ResourceCheckingLock("a-name")})
				Ω(err).ShouldNot(HaveOccurred())

				secondLockCh := make(chan db.Lock, 1)

				go func() {
					defer GinkgoRecover()

					secondLock, err := database.AcquireReadLock([]db.NamedLock{db.ResourceCheckingLock("a-name")})
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
				lock, err := database.AcquireWriteLock([]db.NamedLock{db.ResourceCheckingLock("name-1")})
				Ω(err).ShouldNot(HaveOccurred())

				var secondLock db.Lock
				secondLockCh := make(chan db.Lock, 1)

				go func() {
					defer GinkgoRecover()

					secondLock, err := database.AcquireWriteLock([]db.NamedLock{db.ResourceCheckingLock("name-2")})
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
				lock, err := database.AcquireWriteLock([]db.NamedLock{db.ResourceCheckingLock("name-1"), db.ResourceCheckingLock("name-2")})
				Ω(err).ShouldNot(HaveOccurred())

				secondLockCh := make(chan db.Lock, 1)

				go func() {
					defer GinkgoRecover()

					secondLock, err := database.AcquireWriteLock([]db.NamedLock{db.ResourceCheckingLock("name-1")})
					Ω(err).ShouldNot(HaveOccurred())

					secondLockCh <- secondLock
				}()

				Consistently(secondLockCh).ShouldNot(Receive())

				thirdLockCh := make(chan db.Lock, 1)

				go func() {
					defer GinkgoRecover()

					thirdLock, err := database.AcquireWriteLock([]db.NamedLock{db.ResourceCheckingLock("name-2")})
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
				lock, err := database.AcquireWriteLock([]db.NamedLock{db.ResourceCheckingLock("a-name")})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(database.ListLocks()).Should(ContainElement(db.ResourceCheckingLock("a-name").Name()))

				secondLockCh := make(chan db.Lock, 1)

				go func() {
					defer GinkgoRecover()

					secondLock, err := database.AcquireWriteLock([]db.NamedLock{db.ResourceCheckingLock("a-name")})
					Ω(err).ShouldNot(HaveOccurred())

					secondLockCh <- secondLock
				}()

				Consistently(secondLockCh).ShouldNot(Receive())

				err = lock.Release()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(database.ListLocks()).Should(ContainElement(db.ResourceCheckingLock("a-name").Name()))

				var secondLock db.Lock
				Eventually(secondLockCh).Should(Receive(&secondLock))

				err = secondLock.Release()
				Ω(err).ShouldNot(HaveOccurred())

				Ω(database.ListLocks()).Should(BeEmpty())
			})
		})
	}
}
