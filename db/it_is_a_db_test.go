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
				Expect(err).NotTo(HaveOccurred())

				err = database.CreatePipe(myGuid.String(), "a-url")
				Expect(err).NotTo(HaveOccurred())

				pipe, err := database.GetPipe(myGuid.String())
				Expect(err).NotTo(HaveOccurred())
				Expect(pipe.ID).To(Equal(myGuid.String()))
				Expect(pipe.URL).To(Equal("a-url"))
			})
		})

		It("can get a build's inputs", func() {
			build, err := database.PipelineDB.CreateJobBuild("some-job")
			Expect(err).ToNot(HaveOccurred())

			expectedBuildInput, err := database.PipelineDB.SaveBuildInput(build.ID, db.BuildInput{
				Name: "some-input",
				VersionedResource: db.VersionedResource{
					Resource: "some-resource",
					Type:     "some-type",
					Version: db.Version{
						"some": "version",
					},
					Metadata: []db.MetadataField{
						{
							Name:  "meta1",
							Value: "data1",
						},
						{
							Name:  "meta2",
							Value: "data2",
						},
					},
					PipelineName: "some-pipeline",
				},
			})
			Expect(err).ToNot(HaveOccurred())

			actualBuildInput, err := database.DB.GetBuildInputVersionedResouces(build.ID)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(actualBuildInput)).To(Equal(1))
			Expect(actualBuildInput[0]).To(Equal(expectedBuildInput))
		})

		It("can get a build's output", func() {
			build, err := database.PipelineDB.CreateJobBuild("some-job")
			Expect(err).ToNot(HaveOccurred())

			expectedBuildOutput, err := database.PipelineDB.SaveBuildOutput(build.ID, db.VersionedResource{
				Resource: "some-explicit-resource",
				Type:     "some-type",
				Version: db.Version{
					"some": "version",
				},
				Metadata: []db.MetadataField{
					{
						Name:  "meta1",
						Value: "data1",
					},
					{
						Name:  "meta2",
						Value: "data2",
					},
				},
				PipelineName: "some-pipeline",
			}, true)

			_, err = database.PipelineDB.SaveBuildOutput(build.ID, db.VersionedResource{
				Resource: "some-implicit-resource",
				Type:     "some-type",
				Version: db.Version{
					"some": "version",
				},
				Metadata: []db.MetadataField{
					{
						Name:  "meta1",
						Value: "data1",
					},
					{
						Name:  "meta2",
						Value: "data2",
					},
				},
				PipelineName: "some-pipeline",
			}, false)
			Expect(err).ToNot(HaveOccurred())

			actualBuildOutput, err := database.DB.GetBuildOutputVersionedResouces(build.ID)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(actualBuildOutput)).To(Equal(1))
			Expect(actualBuildOutput[0]).To(Equal(expectedBuildOutput))
		})

		It("can keep track of volume data", func() {
			By("allowing you to insert")
			expectedVolume := db.Volume{
				WorkerName:      "some-worker",
				TTL:             time.Hour,
				Handle:          "some-volume-handle",
				ResourceVersion: atc.Version{"some": "version"},
				ResourceHash:    "some-hash",
			}
			err := database.InsertVolume(expectedVolume)
			Expect(err).NotTo(HaveOccurred())

			By("getting volume information from the db")
			volumes, err := database.GetVolumes()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(volumes)).To(Equal(1))
			actualVolume := volumes[0]
			Expect(actualVolume.WorkerName).To(Equal(expectedVolume.WorkerName))
			Expect(actualVolume.TTL).To(Equal(expectedVolume.TTL))
			Expect(actualVolume.ExpiresIn).To(BeNumerically("~", expectedVolume.TTL, time.Second))
			Expect(actualVolume.Handle).To(Equal(expectedVolume.Handle))
			Expect(actualVolume.ResourceVersion).To(Equal(expectedVolume.ResourceVersion))
			Expect(actualVolume.ResourceHash).To(Equal(expectedVolume.ResourceHash))

			By("allowing you to call insert idempotently")
			err = database.InsertVolume(expectedVolume)
			Expect(err).NotTo(HaveOccurred())

			By("not returning volumes that have expired")
			err = database.InsertVolume(db.Volume{
				WorkerName:      "some-worker",
				TTL:             -time.Hour,
				Handle:          "some-other-volume-handle",
				ResourceVersion: atc.Version{"some": "version"},
				ResourceHash:    "some-hash",
			})
			Expect(err).NotTo(HaveOccurred())

			volumes, err = database.GetVolumes()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(volumes)).To(Equal(1))

			By("allowing you to insert the same volume handle on a different worker")
			err = database.InsertVolume(db.Volume{
				WorkerName:      "some-other-worker",
				TTL:             time.Hour,
				Handle:          "some-volume-handle",
				ResourceVersion: atc.Version{"some": "version"},
				ResourceHash:    "some-hash",
			})
			Expect(err).NotTo(HaveOccurred())
			volumes, err = database.GetVolumes()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(volumes)).To(Equal(2))

			By("letting you get the ttl of a volume")
			actualTTL, err := database.GetVolumeTTL(actualVolume.Handle)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualTTL).To(Equal(actualVolume.TTL))

			By("letting you update the ttl of the volume data")
			err = database.SetVolumeTTL(actualVolume, -time.Hour)
			Expect(err).NotTo(HaveOccurred())
			volumes, err = database.GetVolumes()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(volumes)).To(Equal(1))
		})

		It("saves and propagates events correctly", func() {
			build, err := database.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
			Expect(build.Name).To(Equal("1"))

			By("allowing you to subscribe when no events have yet occurred")
			events, err := database.GetBuildEvents(build.ID, 0)
			Expect(err).NotTo(HaveOccurred())

			defer events.Close()

			By("saving them in order")
			err = database.SaveBuildEvent(build.ID, event.Log{
				Payload: "some ",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(events.Next()).To(Equal(event.Log{
				Payload: "some ",
			}))

			err = database.SaveBuildEvent(build.ID, event.Log{
				Payload: "log",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(events.Next()).To(Equal(event.Log{
				Payload: "log",
			}))

			By("allowing you to subscribe from an offset")
			eventsFrom1, err := database.GetBuildEvents(build.ID, 1)
			Expect(err).NotTo(HaveOccurred())

			defer eventsFrom1.Close()

			Expect(eventsFrom1.Next()).To(Equal(event.Log{
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
			Expect(err).NotTo(HaveOccurred())

			Eventually(nextEvent).Should(Receive(Equal(event.Log{
				Payload: "log 2",
			})))

			By("returning ErrBuildEventStreamClosed for Next calls after Close")
			events3, err := database.GetBuildEvents(build.ID, 0)
			Expect(err).NotTo(HaveOccurred())

			err = events3.Close()
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				_, err := events3.Next()
				return err
			}).Should(Equal(db.ErrBuildEventStreamClosed))
		})

		It("saves and emits status events", func() {
			build, err := database.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
			Expect(build.Name).To(Equal("1"))

			By("allowing you to subscribe when no events have yet occurred")
			events, err := database.GetBuildEvents(build.ID, 0)
			Expect(err).NotTo(HaveOccurred())

			defer events.Close()

			By("emitting a status event when started")
			started, err := database.StartBuild(build.ID, "engine", "metadata")
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())

			startedBuild, found, err := database.GetBuild(build.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(events.Next()).To(Equal(event.Status{
				Status: atc.StatusStarted,
				Time:   startedBuild.StartTime.Unix(),
			}))

			By("emitting a status event when finished")
			err = database.FinishBuild(build.ID, db.StatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			finishedBuild, found, err := database.GetBuild(build.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(events.Next()).To(Equal(event.Status{
				Status: atc.StatusSucceeded,
				Time:   finishedBuild.EndTime.Unix(),
			}))

			By("ending the stream when finished")
			_, err = events.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))
		})

		It("can keep track of workers", func() {
			Expect(database.Workers()).To(BeEmpty())

			infoA := db.WorkerInfo{
				Name:             "workerName1",
				GardenAddr:       "1.2.3.4:7777",
				BaggageclaimURL:  "5.6.7.8:7788",
				ActiveContainers: 42,
				ResourceTypes: []atc.WorkerResourceType{
					{Type: "some-resource-a", Image: "some-image-a"},
				},
				Platform: "webos",
				Tags:     []string{"palm", "was", "great"},
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
			Expect(err).NotTo(HaveOccurred())

			Expect(database.Workers()).To(ConsistOf(infoA))

			By("being idempotent")
			err = database.SaveWorker(infoA, 0)
			Expect(err).NotTo(HaveOccurred())

			Expect(database.Workers()).To(ConsistOf(infoA))

			By("updating attributes by name")
			infoA.GardenAddr = "1.2.3.4:9876"

			err = database.SaveWorker(infoA, 0)
			Expect(err).NotTo(HaveOccurred())

			Expect(database.Workers()).To(ConsistOf(infoA))

			By("updating attributes by address")
			infoA.Name = "someNewName"

			err = database.SaveWorker(infoA, 0)
			Expect(err).NotTo(HaveOccurred())

			Expect(database.Workers()).To(ConsistOf(infoA))

			By("expiring TTLs")
			ttl := 1 * time.Second

			err = database.SaveWorker(infoB, ttl)
			Expect(err).NotTo(HaveOccurred())

			// name is defaulted to addr
			infoBFromDB := infoB
			infoBFromDB.Name = "1.2.3.4:8888"

			Consistently(database.Workers, ttl/2).Should(ConsistOf(infoA, infoBFromDB))
			Eventually(database.Workers, 2*ttl).Should(ConsistOf(infoA))

			By("overwriting TTLs")
			err = database.SaveWorker(infoA, ttl)
			Expect(err).NotTo(HaveOccurred())

			Consistently(database.Workers, ttl/2).Should(ConsistOf(infoA))
			Eventually(database.Workers, 2*ttl).Should(BeEmpty())

			By("updating attributes by name with ttls")
			ttl = 1 * time.Hour
			err = database.SaveWorker(infoA, ttl)
			Expect(err).NotTo(HaveOccurred())

			Expect(database.Workers()).To(ConsistOf(infoA))

			infoA.GardenAddr = "1.2.3.4:1234"

			err = database.SaveWorker(infoA, ttl)
			Expect(err).NotTo(HaveOccurred())

			Expect(database.Workers()).To(ConsistOf(infoA))
		})

		It("it can keep track of a worker", func() {
			By("calling it with worker names that do not exist")

			workerInfo, found, err := database.GetWorker("nope")
			Expect(err).NotTo(HaveOccurred())
			Expect(workerInfo).To(Equal(db.WorkerInfo{}))
			Expect(found).To(BeFalse())

			infoA := db.WorkerInfo{
				GardenAddr:       "1.2.3.4:7777",
				BaggageclaimURL:  "http://5.6.7.8:7788",
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
				BaggageclaimURL:  "http://5.6.7.8:8899",
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
				BaggageclaimURL:  "http://5.6.7.9:8899",
				ActiveContainers: 42,
				ResourceTypes: []atc.WorkerResourceType{
					{Type: "some-resource-b", Image: "some-image-b"},
				},
				Platform: "plan9",
				Tags:     []string{"russ", "cox", "was", "here"},
			}

			err = database.SaveWorker(infoA, 0)
			Expect(err).NotTo(HaveOccurred())

			err = database.SaveWorker(infoB, 0)
			Expect(err).NotTo(HaveOccurred())

			err = database.SaveWorker(infoC, 0)
			Expect(err).NotTo(HaveOccurred())

			By("returning one workerinfo by worker name")
			workerInfo, found, err = database.GetWorker("workerName2")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(workerInfo).To(Equal(infoB))

			By("returning one workerinfo by addr if name is null")
			workerInfo, found, err = database.GetWorker("1.2.3.5:8888")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(workerInfo.Name).To(Equal("1.2.3.5:8888"))

			By("expiring TTLs")
			ttl := 1 * time.Second

			err = database.SaveWorker(infoA, ttl)
			Expect(err).NotTo(HaveOccurred())

			workerFound := func() bool {
				_, found, _ = database.GetWorker("workerName1")
				return found
			}

			Consistently(workerFound, ttl/2).Should(BeTrue())
			Eventually(workerFound, 2*ttl).Should(BeFalse())
		})

		It("can create and get a container info object", func() {
			expectedContainer := db.Container{
				ContainerIdentifier: db.ContainerIdentifier{
					Name:                 "some-container",
					PipelineName:         "some-pipeline",
					BuildID:              123,
					Type:                 db.ContainerTypeTask,
					WorkerName:           "some-worker",
					WorkingDirectory:     "tmp/build/some-guid",
					CheckType:            "some-type",
					CheckSource:          atc.Source{"uri": "http://example.com"},
					StepLocation:         456,
					EnvironmentVariables: []string{"VAR1=val1", "VAR2=val2"},
				},
				Handle: "some-handle",
			}

			By("creating a container")
			err := database.CreateContainer(expectedContainer, time.Minute)
			Expect(err).NotTo(HaveOccurred())

			By("trying to create a container with the same handle")
			err = database.CreateContainer(db.Container{Handle: "some-handle"}, time.Second)
			Expect(err).To(HaveOccurred())

			By("getting the saved info object by h andle")
			actualContainer, found, err := database.GetContainer("some-handle")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(actualContainer.Handle).To(Equal("some-handle"))
			Expect(actualContainer.Name).To(Equal("some-container"))
			Expect(actualContainer.PipelineName).To(Equal("some-pipeline"))
			Expect(actualContainer.BuildID).To(Equal(123))
			Expect(actualContainer.Type).To(Equal(db.ContainerTypeTask))
			Expect(actualContainer.WorkerName).To(Equal("some-worker"))
			Expect(actualContainer.WorkingDirectory).To(Equal("tmp/build/some-guid"))
			Expect(actualContainer.CheckType).To(Equal("some-type"))
			Expect(actualContainer.CheckSource).To(Equal(atc.Source{"uri": "http://example.com"}))
			Expect(actualContainer.StepLocation).To(Equal(uint(456)))
			Expect(actualContainer.EnvironmentVariables).To(Equal([]string{"VAR1=val1", "VAR2=val2"}))

			By("returning found = false when getting by a handle that does not exist")
			_, found, err = database.GetContainer("nope")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})

		It("can update the time to live for a container info object", func() {
			updatedTTL := 5 * time.Minute

			originalContainer := db.Container{
				ContainerIdentifier: db.ContainerIdentifier{
					Type: db.ContainerTypeTask,
				},
				Handle: "some-handle",
			}
			err := database.CreateContainer(originalContainer, time.Minute)
			Expect(err).NotTo(HaveOccurred())

			// comparisonContainer is used to get the expected expiration time in the
			// database timezone to avoid timezone errors
			comparisonContainer := db.Container{
				ContainerIdentifier: db.ContainerIdentifier{
					Type: db.ContainerTypeTask,
				},
				Handle: "comparison-handle",
			}
			err = database.CreateContainer(comparisonContainer, updatedTTL)
			Expect(err).NotTo(HaveOccurred())

			comparisonContainer, found, err := database.GetContainer("comparison-handle")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			err = database.UpdateExpiresAtOnContainer("some-handle", updatedTTL)
			Expect(err).NotTo(HaveOccurred())

			updatedContainer, found, err := database.GetContainer("some-handle")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(updatedContainer.ExpiresAt).To(BeTemporally("~", comparisonContainer.ExpiresAt, time.Second))
		})

		It("can reap a container", func() {
			info := db.Container{
				ContainerIdentifier: db.ContainerIdentifier{
					Type: db.ContainerTypeTask,
				},
				Handle: "some-handle",
			}

			err := database.CreateContainer(info, time.Minute)
			Expect(err).NotTo(HaveOccurred())

			_, found, err := database.GetContainer("some-handle")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			By("reaping an existing container")
			err = database.ReapContainer("some-handle")
			Expect(err).NotTo(HaveOccurred())

			_, found, err = database.GetContainer("some-handle")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())

			By("not failing if the container's already been reaped")
			err = database.ReapContainer("some-handle")
			Expect(err).NotTo(HaveOccurred())
		})

		type findContainersByIdentifierExample struct {
			containersToCreate   []db.Container
			identifierToFilerFor db.ContainerIdentifier
			expectedHandles      []string
		}

		DescribeTable("filtering containers by identifier",
			func(example findContainersByIdentifierExample) {
				var results []db.Container
				var handles []string
				var err error

				for _, containerToCreate := range example.containersToCreate {
					if containerToCreate.Type.String() == "" {
						containerToCreate.Type = db.ContainerTypeTask
					}

					err = database.CreateContainer(containerToCreate, 1*time.Minute)
					Expect(err).NotTo(HaveOccurred())
				}

				results, err = database.FindContainersByIdentifier(example.identifierToFilerFor)
				Expect(err).NotTo(HaveOccurred())

				for _, result := range results {
					handles = append(handles, result.Handle)
				}

				Expect(handles).To(ConsistOf(example.expectedHandles))

				for _, containerToDelete := range example.containersToCreate {
					err = database.DeleteContainer(containerToDelete.Handle)
					Expect(err).NotTo(HaveOccurred())
				}
			},

			Entry("returns everything when no filters are passed", findContainersByIdentifierExample{
				containersToCreate: []db.Container{
					{Handle: "a"},
					{Handle: "b"},
				},
				identifierToFilerFor: db.ContainerIdentifier{},
				expectedHandles:      []string{"a", "b"},
			}),

			Entry("does not return things that the filter doesn't match", findContainersByIdentifierExample{
				containersToCreate: []db.Container{
					{Handle: "a"},
					{Handle: "b"},
				},
				identifierToFilerFor: db.ContainerIdentifier{Name: "some-name"},
				expectedHandles:      nil,
			}),

			Entry("returns containers where the name matches", findContainersByIdentifierExample{
				containersToCreate: []db.Container{
					{Handle: "a", ContainerIdentifier: db.ContainerIdentifier{Name: "some-container"}},
					{Handle: "b", ContainerIdentifier: db.ContainerIdentifier{Name: "some-container"}},
					{Handle: "c", ContainerIdentifier: db.ContainerIdentifier{Name: "some-other"}},
				},
				identifierToFilerFor: db.ContainerIdentifier{Name: "some-container"},
				expectedHandles:      []string{"a", "b"},
			}),

			Entry("returns containers where the pipeline matches", findContainersByIdentifierExample{
				containersToCreate: []db.Container{
					{Handle: "a", ContainerIdentifier: db.ContainerIdentifier{PipelineName: "some-pipeline"}},
					{Handle: "b", ContainerIdentifier: db.ContainerIdentifier{PipelineName: "some-other"}},
					{Handle: "c", ContainerIdentifier: db.ContainerIdentifier{PipelineName: "some-pipeline"}},
				},
				identifierToFilerFor: db.ContainerIdentifier{PipelineName: "some-pipeline"},
				expectedHandles:      []string{"a", "c"},
			}),

			Entry("returns containers where the build id matches", findContainersByIdentifierExample{
				containersToCreate: []db.Container{
					{Handle: "a", ContainerIdentifier: db.ContainerIdentifier{BuildID: 1}},
					{Handle: "b", ContainerIdentifier: db.ContainerIdentifier{BuildID: 2}},
					{Handle: "c", ContainerIdentifier: db.ContainerIdentifier{BuildID: 2}},
				},
				identifierToFilerFor: db.ContainerIdentifier{BuildID: 2},
				expectedHandles:      []string{"b", "c"},
			}),

			Entry("returns containers where the type matches", findContainersByIdentifierExample{
				containersToCreate: []db.Container{
					{Handle: "a", ContainerIdentifier: db.ContainerIdentifier{Type: db.ContainerTypePut}},
					{Handle: "b", ContainerIdentifier: db.ContainerIdentifier{Type: db.ContainerTypePut}},
					{Handle: "c", ContainerIdentifier: db.ContainerIdentifier{Type: db.ContainerTypeGet}},
				},
				identifierToFilerFor: db.ContainerIdentifier{Type: db.ContainerTypePut},
				expectedHandles:      []string{"a", "b"},
			}),

			Entry("returns containers where the worker name matches", findContainersByIdentifierExample{
				containersToCreate: []db.Container{
					{Handle: "a", ContainerIdentifier: db.ContainerIdentifier{WorkerName: "some-worker"}},
					{Handle: "b", ContainerIdentifier: db.ContainerIdentifier{WorkerName: "some-worker"}},
					{Handle: "c", ContainerIdentifier: db.ContainerIdentifier{WorkerName: "other"}},
				},
				identifierToFilerFor: db.ContainerIdentifier{WorkerName: "some-worker"},
				expectedHandles:      []string{"a", "b"},
			}),

			Entry("returns containers where the check type matches", findContainersByIdentifierExample{
				containersToCreate: []db.Container{
					{Handle: "a", ContainerIdentifier: db.ContainerIdentifier{CheckType: "some-type"}},
					{Handle: "b", ContainerIdentifier: db.ContainerIdentifier{CheckType: "nope"}},
					{Handle: "c", ContainerIdentifier: db.ContainerIdentifier{CheckType: "some-type"}},
				},
				identifierToFilerFor: db.ContainerIdentifier{CheckType: "some-type"},
				expectedHandles:      []string{"a", "c"},
			}),

			Entry("returns containers where the check source matches", findContainersByIdentifierExample{
				containersToCreate: []db.Container{
					{Handle: "a", ContainerIdentifier: db.ContainerIdentifier{CheckSource: atc.Source{"some": "other-source"}}},
					{Handle: "b", ContainerIdentifier: db.ContainerIdentifier{CheckSource: atc.Source{"some": "source"}}},
					{Handle: "c", ContainerIdentifier: db.ContainerIdentifier{CheckSource: atc.Source{"some": "source"}}},
				},
				identifierToFilerFor: db.ContainerIdentifier{CheckSource: atc.Source{"some": "source"}},
				expectedHandles:      []string{"b", "c"},
			}),

			Entry("returns containers where the step location matches", findContainersByIdentifierExample{
				containersToCreate: []db.Container{
					{Handle: "a", ContainerIdentifier: db.ContainerIdentifier{StepLocation: 123}},
					{Handle: "b", ContainerIdentifier: db.ContainerIdentifier{StepLocation: 123}},
					{Handle: "c", ContainerIdentifier: db.ContainerIdentifier{StepLocation: 456}},
				},
				identifierToFilerFor: db.ContainerIdentifier{StepLocation: 123},
				expectedHandles:      []string{"a", "b"},
			}),

			Entry("returns containers where all fields match", findContainersByIdentifierExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Name:         "some-name",
							PipelineName: "some-pipeline",
							BuildID:      123,
							Type:         db.ContainerTypeCheck,
							WorkerName:   "some-worker",
						},
						Handle: "a",
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Name:         "WROONG",
							PipelineName: "some-pipeline",
							BuildID:      123,
							Type:         db.ContainerTypeCheck,
							WorkerName:   "some-worker",
						},
						Handle: "b",
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Name:         "some-name",
							PipelineName: "some-pipeline",
							BuildID:      123,
							Type:         db.ContainerTypeCheck,
							WorkerName:   "some-worker",
						},
						Handle: "c",
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							WorkerName: "Wat",
						},
						Handle: "d",
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
			expectedContainer := db.Container{
				Handle: "some-handle",
				ContainerIdentifier: db.ContainerIdentifier{
					PipelineName: "some-pipeline",
					BuildID:      123,
					Name:         "some-container",
					WorkerName:   "some-worker",
					Type:         db.ContainerTypeTask,
					CheckType:    "some-type",
					CheckSource:  atc.Source{"some": "other-source"},
				},
			}
			otherContainer := db.Container{
				Handle: "other-handle",
				ContainerIdentifier: db.ContainerIdentifier{
					Name: "other-container",
					Type: db.ContainerTypeTask,
				},
			}

			err := database.CreateContainer(expectedContainer, time.Minute)
			Expect(err).NotTo(HaveOccurred())
			err = database.CreateContainer(otherContainer, time.Minute)
			Expect(err).NotTo(HaveOccurred())

			By("returning a single matching container info")
			actualContainer, found, err := database.FindContainerByIdentifier(db.ContainerIdentifier{Name: "some-container"})

			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(actualContainer.Handle).To(Equal("some-handle"))
			Expect(actualContainer.Name).To(Equal("some-container"))
			Expect(actualContainer.PipelineName).To(Equal("some-pipeline"))
			Expect(actualContainer.BuildID).To(Equal(123))
			Expect(actualContainer.Type).To(Equal(db.ContainerTypeTask))
			Expect(actualContainer.WorkerName).To(Equal("some-worker"))
			Expect(actualContainer.CheckType).To(Equal("some-type"))
			Expect(actualContainer.CheckSource).To(Equal(atc.Source{"some": "other-source"}))
			Expect(actualContainer.ExpiresAt.String()).NotTo(BeEmpty())

			By("erroring if more than one container matches the filter")
			actualContainer, found, err = database.FindContainerByIdentifier(db.ContainerIdentifier{Type: db.ContainerTypeTask})
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(db.ErrMultipleContainersFound))
			Expect(found).To(BeFalse())
			Expect(actualContainer.Handle).To(BeEmpty())

			By("returning found of false if no containers match the filter")
			actualContainer, found, err = database.FindContainerByIdentifier(db.ContainerIdentifier{Name: "nope"})
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
			Expect(actualContainer.Handle).To(BeEmpty())

			By("removing it if the TTL has expired")
			ttl := 1 * time.Second
			ttlContainer := db.Container{
				Handle: "some-ttl-handle",
				ContainerIdentifier: db.ContainerIdentifier{
					Name: "some-ttl-name",
					Type: db.ContainerTypeTask,
				},
			}

			err = database.CreateContainer(ttlContainer, -ttl)
			Expect(err).NotTo(HaveOccurred())
			_, found, err = database.FindContainerByIdentifier(db.ContainerIdentifier{Name: "some-ttl-name"})
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})

		It("can create one-off builds with increasing names", func() {
			oneOff, err := database.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
			Expect(oneOff.ID).NotTo(BeZero())
			Expect(oneOff.JobName).To(BeZero())
			Expect(oneOff.Name).To(Equal("1"))
			Expect(oneOff.Status).To(Equal(db.StatusPending))

			oneOffGot, found, err := database.GetBuild(oneOff.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(oneOffGot).To(Equal(oneOff))

			jobBuild, err := database.PipelineDB.CreateJobBuild("some-other-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(jobBuild.Name).To(Equal("1"))

			nextOneOff, err := database.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
			Expect(nextOneOff.ID).NotTo(BeZero())
			Expect(nextOneOff.ID).NotTo(Equal(oneOff.ID))
			Expect(nextOneOff.JobName).To(BeZero())
			Expect(nextOneOff.Name).To(Equal("2"))
			Expect(nextOneOff.Status).To(Equal(db.StatusPending))

			allBuilds, err := database.GetAllBuilds()
			Expect(err).NotTo(HaveOccurred())
			Expect(allBuilds).To(Equal([]db.Build{nextOneOff, jobBuild, oneOff}))
		})

		Describe("GetAllStartedBuilds", func() {
			var build1 db.Build
			var build2 db.Build
			BeforeEach(func() {
				var err error

				build1, err = database.CreateOneOffBuild()
				Expect(err).NotTo(HaveOccurred())

				build2, err = database.PipelineDB.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())

				_, err = database.CreateOneOffBuild()
				Expect(err).NotTo(HaveOccurred())

				started, err := database.StartBuild(build1.ID, "some-engine", "so-meta")
				Expect(err).NotTo(HaveOccurred())
				Expect(started).To(BeTrue())

				started, err = database.StartBuild(build2.ID, "some-engine", "so-meta")
				Expect(err).NotTo(HaveOccurred())
				Expect(started).To(BeTrue())
			})

			It("returns all builds that have been started, regardless of pipeline", func() {
				builds, err := database.GetAllStartedBuilds()
				Expect(err).NotTo(HaveOccurred())

				Expect(len(builds)).To(Equal(2))

				build1, found, err := database.GetBuild(build1.ID)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				build2, found, err := database.GetBuild(build2.ID)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(builds).To(ConsistOf(build1, build2))
			})
		})
	}
}

type someLock string

func (lock someLock) Name() string {
	return "some-lock:" + string(lock)
}
