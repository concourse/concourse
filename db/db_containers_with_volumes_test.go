package db_test

import (
	"time"

	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

var _ = Describe("Keeping track of containers", func() {
	var (
		dbConn   db.Conn
		listener *pq.Listener

		database db.DB
		teamDB   db.TeamDB
		teamID   int
	)

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.Open())

		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		lockFactory := db.NewLockFactory(postgresRunner.OpenPgx())
		database = db.NewSQL(dbConn, bus, lockFactory)
		teamDBFactory := db.NewTeamDBFactory(dbConn, bus, lockFactory)

		savedTeam, err := database.CreateTeam(db.Team{Name: "some-team"})
		Expect(err).NotTo(HaveOccurred())
		teamID = savedTeam.ID

		teamDB = teamDBFactory.GetTeamDB("some-team")
	})

	getVolumes := func() map[string]db.SavedVolume {
		volumes, err := database.GetVolumes()
		Expect(err).NotTo(HaveOccurred())
		result := map[string]db.SavedVolume{}
		for _, volume := range volumes {
			result[volume.Handle] = volume
		}
		return result
	}

	Describe("CreateContainer", func() {
		Context("when creating a container with volumes", func() {
			It("sets ContainerTTL on each volume", func() {
				someBuild, err := teamDB.CreateOneOffBuild()
				Expect(err).ToNot(HaveOccurred())

				volume1 := db.Volume{
					Handle: "volume-1-handle",
				}
				err = database.InsertVolume(volume1)
				Expect(err).NotTo(HaveOccurred())

				volume2 := db.Volume{
					Handle: "volume-2-handle",
				}
				err = database.InsertVolume(volume2)
				Expect(err).NotTo(HaveOccurred())

				volumesMap := getVolumes()

				container1 := db.Container{
					ContainerIdentifier: db.ContainerIdentifier{
						BuildID: someBuild.ID(),
						PlanID:  atc.PlanID("some-task"),
						Stage:   db.ContainerStageRun,
					},
					ContainerMetadata: db.ContainerMetadata{
						Handle: "some-handle-1",
						Type:   db.ContainerTypeTask,
						TeamID: teamID,
					},
				}
				savedContainer1, err := database.CreateContainer(container1, 5*time.Minute, 0, []string{
					volume1.Handle,
					volume2.Handle,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(savedContainer1.Handle).To(Equal("some-handle-1"))

				volumesMap = getVolumes()

				Expect(volumesMap[volume1.Handle].ContainerTTL).NotTo(BeNil())
				Expect(*volumesMap[volume1.Handle].ContainerTTL).To(Equal(5 * time.Minute))

				Expect(volumesMap[volume2.Handle].ContainerTTL).NotTo(BeNil())
				Expect(*volumesMap[volume2.Handle].ContainerTTL).To(Equal(5 * time.Minute))

				By("updating the container id for volumes that already have one")
				container2 := db.Container{
					ContainerIdentifier: db.ContainerIdentifier{
						BuildID: someBuild.ID(),
						PlanID:  atc.PlanID("some-task"),
						Stage:   db.ContainerStageRun,
					},
					ContainerMetadata: db.ContainerMetadata{
						Handle: "some-handle-2",
						Type:   db.ContainerTypeTask,
						TeamID: teamID,
					},
				}
				savedContainer2, err := database.CreateContainer(container2, 19*time.Minute, 0, []string{
					volume1.Handle,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(savedContainer2.Handle).To(Equal("some-handle-2"))

				volumesMap = getVolumes()

				Expect(volumesMap[volume1.Handle].ContainerTTL).NotTo(BeNil())
				Expect(*volumesMap[volume1.Handle].ContainerTTL).To(Equal(19 * time.Minute))

				Expect(volumesMap[volume2.Handle].ContainerTTL).NotTo(BeNil())
				Expect(*volumesMap[volume2.Handle].ContainerTTL).To(Equal(5 * time.Minute))

				By("removing the container id from all volumes when the container is reaped")
				err = database.ReapContainer("some-handle-1")
				Expect(err).NotTo(HaveOccurred())

				volumesMap = getVolumes()

				Expect(volumesMap[volume1.Handle].ContainerTTL).NotTo(BeNil())
				Expect(*volumesMap[volume1.Handle].ContainerTTL).To(Equal(19 * time.Minute))

				Expect(volumesMap[volume2.Handle].ContainerTTL).To(BeNil())

				By("removing the container id from all volumes when the container is deleted")
				savedContainer1, err = database.CreateContainer(container1, 8*time.Minute, 0, []string{
					volume2.Handle,
				})
				Expect(err).NotTo(HaveOccurred())
				volumesMap = getVolumes()
				Expect(volumesMap[volume2.Handle].ContainerTTL).NotTo(BeNil())
				Expect(*volumesMap[volume2.Handle].ContainerTTL).To(Equal(8 * time.Minute))

				err = database.DeleteContainer("some-handle-1")
				Expect(err).NotTo(HaveOccurred())

				volumesMap = getVolumes()

				Expect(volumesMap[volume1.Handle].ContainerTTL).NotTo(BeNil())
				Expect(*volumesMap[volume1.Handle].ContainerTTL).To(Equal(19 * time.Minute))

				Expect(volumesMap[volume2.Handle].ContainerTTL).To(BeNil())
			})
		})
	})

	Describe("GetContainer", func() {
		Context("when a container has expired", func() {
			It("deletes the container and sets its volumes' container_id to null", func() {
				someBuild, err := teamDB.CreateOneOffBuild()
				Expect(err).ToNot(HaveOccurred())

				container := db.Container{
					ContainerIdentifier: db.ContainerIdentifier{
						BuildID: someBuild.ID(),
						PlanID:  atc.PlanID("some-task"),
						Stage:   db.ContainerStageRun,
					},
					ContainerMetadata: db.ContainerMetadata{
						Handle: "some-handle-1",
						Type:   db.ContainerTypeTask,
						TeamID: teamID,
					},
				}

				expiredContainer := db.Container{
					ContainerIdentifier: db.ContainerIdentifier{
						BuildID: someBuild.ID(),
						PlanID:  atc.PlanID("some-task"),
						Stage:   db.ContainerStageRun,
					},
					ContainerMetadata: db.ContainerMetadata{
						Handle: "some-handle-2",
						Type:   db.ContainerTypeTask,
						TeamID: teamID,
					},
				}

				volume1 := db.Volume{
					Handle: "volume-1-handle",
				}
				err = database.InsertVolume(volume1)
				Expect(err).NotTo(HaveOccurred())

				volume2 := db.Volume{
					Handle: "volume-2-handle",
				}
				err = database.InsertVolume(volume2)
				Expect(err).NotTo(HaveOccurred())

				database.CreateContainer(container, 10*time.Minute, 0, []string{"volume-1-handle"})

				database.CreateContainer(expiredContainer, 1*time.Nanosecond, 0, []string{
					"volume-2-handle",
				})

				time.Sleep(2 * time.Nanosecond)

				_, found, err := database.GetContainer("some-handle-2")
				Expect(found).To(BeFalse())
				Expect(err).NotTo(HaveOccurred())

				volumeMap := getVolumes()
				Expect(volumeMap["volume-1-handle"].ContainerTTL).NotTo(BeNil())
				Expect(*volumeMap["volume-1-handle"].ContainerTTL).To(Equal(10 * time.Minute))
				Expect(volumeMap["volume-2-handle"].ContainerTTL).To(BeNil())
			})
		})
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})
})
