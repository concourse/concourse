package db_test

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
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

			startedBuild, found, err := database.GetBuild(build.ID)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(found).Should(BeTrue())

			Ω(events.Next()).Should(Equal(event.Status{
				Status: atc.StatusStarted,
				Time:   startedBuild.StartTime.Unix(),
			}))

			By("emitting a status event when finished")
			err = database.FinishBuild(build.ID, db.StatusSucceeded)
			Ω(err).ShouldNot(HaveOccurred())

			finishedBuild, found, err := database.GetBuild(build.ID)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(found).Should(BeTrue())

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
				GardenAddr:       "1.2.3.4:7777",
				BaggageclaimURL:  "5.6.7.8:7788",
				ActiveContainers: 42,
				ResourceTypes: []atc.WorkerResourceType{
					{Type: "some-resource-a", Image: "some-image-a"},
				},
				Platform: "webos",
				Tags:     []string{"palm", "was", "great"},
				Name:     "workerName1",
			}

			infoB := db.WorkerInfo{
				GardenAddr:       "1.2.3.4:8888",
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

			infoB.Name = "1.2.3.4:8888"

			Consistently(database.Workers, ttl/2).Should(ConsistOf(infoA, infoB))
			Eventually(database.Workers, 2*ttl).Should(ConsistOf(infoA))

			By("overwriting TTLs")
			err = database.SaveWorker(infoA, ttl)
			Ω(err).ShouldNot(HaveOccurred())

			Consistently(database.Workers, ttl/2).Should(ConsistOf(infoA))
			Eventually(database.Workers, 2*ttl).Should(BeEmpty())
		})

		It("it can keep track of a worker", func() {
			By("calling it with worker names that do not exist")

			workerInfo, found, err := database.GetWorker("nope")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(workerInfo).Should(Equal(db.WorkerInfo{}))
			Ω(found).Should(BeFalse())

			infoA := db.WorkerInfo{
				GardenAddr:       "1.2.3.4:7777",
				ActiveContainers: 42,
				ResourceTypes: []atc.WorkerResourceType{
					{Type: "some-resource-a", Image: "some-image-a"},
				},
				Platform: "webos",
				Tags:     []string{"palm", "was", "great"},
				Name:     "workerName1",
			}

			infoB := db.WorkerInfo{
				GardenAddr:       "1.2.3.4:8888",
				ActiveContainers: 42,
				ResourceTypes: []atc.WorkerResourceType{
					{Type: "some-resource-b", Image: "some-image-b"},
				},
				Platform: "plan9",
				Tags:     []string{"russ", "cox", "was", "here"},
				Name:     "workerName2",
			}

			infoC := db.WorkerInfo{
				GardenAddr:       "1.2.3.5:8888",
				ActiveContainers: 42,
				ResourceTypes: []atc.WorkerResourceType{
					{Type: "some-resource-b", Image: "some-image-b"},
				},
				Platform: "plan9",
				Tags:     []string{"russ", "cox", "was", "here"},
			}

			err = database.SaveWorker(infoA, 0)
			Ω(err).ShouldNot(HaveOccurred())

			err = database.SaveWorker(infoB, 0)
			Ω(err).ShouldNot(HaveOccurred())

			err = database.SaveWorker(infoC, 0)
			Ω(err).ShouldNot(HaveOccurred())

			By("returning one workerinfo by worker name")
			workerInfo, found, err = database.GetWorker("workerName2")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(found).Should(BeTrue())
			Ω(workerInfo).Should(Equal(infoB))

			By("returning one workerinfo by addr if name is null")
			workerInfo, found, err = database.GetWorker("1.2.3.5:8888")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(found).Should(BeTrue())
			Ω(workerInfo.Name).Should(Equal("1.2.3.5:8888"))

			By("expiring TTLs")
			ttl := 1 * time.Second

			err = database.SaveWorker(infoA, ttl)
			Ω(err).ShouldNot(HaveOccurred())

			workerFound := func() bool {
				_, found, _ = database.GetWorker("workerName1")
				return found
			}

			Consistently(workerFound, ttl/2).Should(BeTrue())
			Eventually(workerFound, 2*ttl).Should(BeFalse())
		})

		It("can create and get a container info object", func() {
			expectedContainerInfo := db.ContainerInfo{
				Handle:       "some-handle",
				Name:         "some-container",
				PipelineName: "some-pipeline",
				BuildID:      123,
				Type:         db.ContainerTypeTask,
				WorkerName:   "some-worker",
			}

			By("creating a container")
			err := database.CreateContainerInfo(expectedContainerInfo, time.Minute)
			Ω(err).ShouldNot(HaveOccurred())

			By("trying to create a container with the same handle")
			err = database.CreateContainerInfo(db.ContainerInfo{Handle: "some-handle"}, time.Second)
			Ω(err).Should(HaveOccurred())

			By("getting the saved info object by handle")
			actualContainerInfo, found, err := database.GetContainerInfo("some-handle")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(found).Should(BeTrue())

			Ω(actualContainerInfo.Handle).Should(Equal("some-handle"))
			Ω(actualContainerInfo.Name).Should(Equal("some-container"))
			Ω(actualContainerInfo.PipelineName).Should(Equal("some-pipeline"))
			Ω(actualContainerInfo.BuildID).Should(Equal(123))
			Ω(actualContainerInfo.Type).Should(Equal(db.ContainerTypeTask))
			Ω(actualContainerInfo.WorkerName).Should(Equal("some-worker"))

			By("returning found = false when getting by a handle that does not exist")
			_, found, err = database.GetContainerInfo("nope")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(found).Should(BeFalse())
		})

		It("can update the time to live for a container info object", func() {
			updatedTTL := 5 * time.Minute

			originalContainerInfo := db.ContainerInfo{
				Handle: "some-handle",
				Type:   db.ContainerTypeTask,
			}
			err := database.CreateContainerInfo(originalContainerInfo, time.Minute)
			Ω(err).ShouldNot(HaveOccurred())

			// comparisonContainerInfo is used to get the expected expiration time in the
			// database timezone to avoid timezone errors
			comparisonContainerInfo := db.ContainerInfo{
				Handle: "comparison-handle",
				Type:   db.ContainerTypeTask,
			}
			err = database.CreateContainerInfo(comparisonContainerInfo, updatedTTL)
			Ω(err).ShouldNot(HaveOccurred())

			comparisonContainerInfo, found, err := database.GetContainerInfo("comparison-handle")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(found).Should(BeTrue())

			err = database.UpdateExpiresAtOnContainerInfo("some-handle", updatedTTL)
			Ω(err).ShouldNot(HaveOccurred())

			updatedContainerInfo, found, err := database.GetContainerInfo("some-handle")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(found).Should(BeTrue())

			Ω(updatedContainerInfo.ExpiresAt).Should(BeTemporally("~", comparisonContainerInfo.ExpiresAt, time.Second))
		})

		type findContainerInfosByIdentifierExample struct {
			containersToCreate   []db.ContainerInfo
			identifierToFilerFor db.ContainerIdentifier
			expectedHandles      []string
		}

		DescribeTable("filtering containers by identifier",
			func(example findContainerInfosByIdentifierExample) {
				var results []db.ContainerInfo
				var handles []string
				var found bool
				var err error

				for _, containerToCreate := range example.containersToCreate {
					if containerToCreate.Type.ToString() == "" {
						containerToCreate.Type = db.ContainerTypeTask
					}

					err = database.CreateContainerInfo(containerToCreate, 1*time.Minute)
					Ω(err).ShouldNot(HaveOccurred())
				}

				results, found, err = database.FindContainerInfosByIdentifier(example.identifierToFilerFor)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(found).Should(Equal(example.expectedHandles != nil))

				for _, result := range results {
					handles = append(handles, result.Handle)
				}

				Ω(handles).Should(ConsistOf(example.expectedHandles))

				for _, containerToDelete := range example.containersToCreate {
					err = database.DeleteContainerInfo(containerToDelete.Handle)
					Ω(err).ShouldNot(HaveOccurred())
				}
			},

			Entry("returns everything when no filters are passed", findContainerInfosByIdentifierExample{
				containersToCreate: []db.ContainerInfo{
					{Handle: "a"},
					{Handle: "b"},
				},
				identifierToFilerFor: db.ContainerIdentifier{},
				expectedHandles:      []string{"a", "b"},
			}),

			Entry("does not return things that the filter doesn't match", findContainerInfosByIdentifierExample{
				containersToCreate: []db.ContainerInfo{
					{Handle: "a"},
					{Handle: "b"},
				},
				identifierToFilerFor: db.ContainerIdentifier{Name: "some-name"},
				expectedHandles:      nil,
			}),

			Entry("returns containers where the name matches", findContainerInfosByIdentifierExample{
				containersToCreate: []db.ContainerInfo{
					{Handle: "a", Name: "some-container"},
					{Handle: "b", Name: "some-container"},
					{Handle: "c", Name: "some-other"},
				},
				identifierToFilerFor: db.ContainerIdentifier{Name: "some-container"},
				expectedHandles:      []string{"a", "b"},
			}),

			Entry("returns containers where the pipeline matches", findContainerInfosByIdentifierExample{
				containersToCreate: []db.ContainerInfo{
					{Handle: "a", PipelineName: "some-pipeline"},
					{Handle: "b", PipelineName: "some-other"},
					{Handle: "c", PipelineName: "some-pipeline"},
				},
				identifierToFilerFor: db.ContainerIdentifier{PipelineName: "some-pipeline"},
				expectedHandles:      []string{"a", "c"},
			}),

			Entry("returns containers where the build id matches", findContainerInfosByIdentifierExample{
				containersToCreate: []db.ContainerInfo{
					{Handle: "a", BuildID: 1},
					{Handle: "b", BuildID: 2},
					{Handle: "c", BuildID: 2},
				},
				identifierToFilerFor: db.ContainerIdentifier{BuildID: 2},
				expectedHandles:      []string{"b", "c"},
			}),

			Entry("returns containers where the type matches", findContainerInfosByIdentifierExample{
				containersToCreate: []db.ContainerInfo{
					{Handle: "a", Type: db.ContainerTypePut},
					{Handle: "b", Type: db.ContainerTypePut},
					{Handle: "c", Type: db.ContainerTypeGet},
				},
				identifierToFilerFor: db.ContainerIdentifier{Type: db.ContainerTypePut},
				expectedHandles:      []string{"a", "b"},
			}),

			Entry("returns containers where the worker name matches", findContainerInfosByIdentifierExample{
				containersToCreate: []db.ContainerInfo{
					{Handle: "a", WorkerName: "some-worker"},
					{Handle: "b", WorkerName: "some-worker"},
					{Handle: "c", WorkerName: "other"},
				},
				identifierToFilerFor: db.ContainerIdentifier{WorkerName: "some-worker"},
				expectedHandles:      []string{"a", "b"},
			}),

			Entry("returns containers where all fields match", findContainerInfosByIdentifierExample{
				containersToCreate: []db.ContainerInfo{
					{
						Handle:       "a",
						Name:         "some-name",
						PipelineName: "some-pipeline",
						BuildID:      123,
						Type:         db.ContainerTypeCheck,
						WorkerName:   "some-worker",
					},
					{
						Handle:       "b",
						Name:         "WROONG",
						PipelineName: "some-pipeline",
						BuildID:      123,
						Type:         db.ContainerTypeCheck,
						WorkerName:   "some-worker",
					},
					{
						Handle:       "c",
						Name:         "some-name",
						PipelineName: "some-pipeline",
						BuildID:      123,
						Type:         db.ContainerTypeCheck,
						WorkerName:   "some-worker"},
					{
						Handle:     "d",
						WorkerName: "Wat",
					},
				},
				identifierToFilerFor: db.ContainerIdentifier{
					Name:         "some-name",
					PipelineName: "some-pipeline",
					BuildID:      123,
					Type:         db.ContainerTypeCheck,
					WorkerName:   "some-worker",
				},
				expectedHandles: []string{"a", "c"},
			}),
		)

		It("can find a single container info by identifier", func() {
			expectedContainerInfo := db.ContainerInfo{
				Handle: "some-handle",
				Name:   "some-container",
				Type:   db.ContainerTypeTask,
			}
			otherContainerInfo := db.ContainerInfo{
				Handle: "other-handle",
				Name:   "other-container",
				Type:   db.ContainerTypeTask,
			}

			err := database.CreateContainerInfo(expectedContainerInfo, time.Minute)
			Ω(err).ShouldNot(HaveOccurred())
			err = database.CreateContainerInfo(otherContainerInfo, time.Minute)
			Ω(err).ShouldNot(HaveOccurred())

			By("returning a single matching container info")
			actualContainerInfo, found, err := database.FindContainerInfoByIdentifier(db.ContainerIdentifier{Name: "some-container"})
			Ω(err).ShouldNot(HaveOccurred())
			Ω(found).Should(BeTrue())
			Ω(actualContainerInfo).Should(Equal(expectedContainerInfo))

			By("erroring if more than one container matches the filter")
			actualContainerInfo, found, err = database.FindContainerInfoByIdentifier(db.ContainerIdentifier{Type: db.ContainerTypeTask})
			Ω(err).Should(HaveOccurred())
			Ω(err).Should(Equal(db.ErrMultipleContainersFound))
			Ω(found).Should(BeFalse())
			Ω(actualContainerInfo.Handle).Should(BeEmpty())

			By("returning found of false if no containers match the filter")
			actualContainerInfo, found, err = database.FindContainerInfoByIdentifier(db.ContainerIdentifier{Name: "nope"})
			Ω(err).ShouldNot(HaveOccurred())
			Ω(found).Should(BeFalse())
			Ω(actualContainerInfo.Handle).Should(BeEmpty())

			By("removing it if the TTL has expired")
			ttl := 1 * time.Second
			ttlContainerInfo := db.ContainerInfo{
				Handle: "some-ttl-handle",
				Name:   "some-ttl-name",
				Type:   db.ContainerTypeTask,
			}

			err = database.CreateContainerInfo(ttlContainerInfo, -ttl)
			Ω(err).ShouldNot(HaveOccurred())
			_, found, err = database.FindContainerInfoByIdentifier(db.ContainerIdentifier{Name: "some-ttl-name"})
			Ω(found).Should(BeFalse())
		})

		It("can create one-off builds with increasing names", func() {
			oneOff, err := database.CreateOneOffBuild()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(oneOff.ID).ShouldNot(BeZero())
			Ω(oneOff.JobName).Should(BeZero())
			Ω(oneOff.Name).Should(Equal("1"))
			Ω(oneOff.Status).Should(Equal(db.StatusPending))

			oneOffGot, found, err := database.GetBuild(oneOff.ID)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(found).Should(BeTrue())
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
			BeforeEach(func() {
				var err error

				build1, err = database.CreateOneOffBuild()
				Ω(err).ShouldNot(HaveOccurred())

				build2, err = database.PipelineDB.CreateJobBuild("some-job")
				Ω(err).ShouldNot(HaveOccurred())

				_, err = database.CreateOneOffBuild()
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

				build1, found, err := database.GetBuild(build1.ID)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(found).Should(BeTrue())
				build2, found, err := database.GetBuild(build2.ID)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(found).Should(BeTrue())

				Ω(builds).Should(ConsistOf(build1, build2))
			})
		})
	}
}

type someLock string

func (lock someLock) Name() string {
	return "some-lock:" + string(lock)
}
