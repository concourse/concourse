package db_test

import (
	"database/sql"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/lib/pq"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Keeping track of volumes", func() {
	var dbConn *sql.DB
	var listener *pq.Listener

	var database db.DB

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = postgresRunner.Open()
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		database = db.NewSQL(lagertest.NewTestLogger("test"), dbConn, bus)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("volume data", func() {
		var (
			volumeToInsert  db.Volume
			workerToInsert  db.WorkerInfo
			workerToInsert2 db.WorkerInfo
			insertedWorker  db.SavedWorker
			insertedWorker2 db.SavedWorker
			err             error
		)

		BeforeEach(func() {
			volumeToInsert = db.Volume{
				TTL:             time.Hour,
				Handle:          "some-volume-handle",
				ResourceVersion: atc.Version{"some": "version"},
				ResourceHash:    "some-hash",
			}
			workerToInsert = db.WorkerInfo{
				GardenAddr:       "some-garden-address",
				BaggageclaimURL:  "some-baggageclaim-url",
				ActiveContainers: 0,
				ResourceTypes:    []atc.WorkerResourceType{},
				Platform:         "linux",
				Tags:             []string{"vsphere"},
				Name:             "some-worker",
			}
			workerToInsert2 = db.WorkerInfo{
				GardenAddr:       "second-garden-address",
				BaggageclaimURL:  "some-baggageclaim-url",
				ActiveContainers: 0,
				ResourceTypes:    []atc.WorkerResourceType{},
				Platform:         "linux",
				Tags:             []string{"vsphere"},
				Name:             "second-worker",
			}
		})

		JustBeforeEach(func() {
			insertedWorker, err = database.SaveWorker(workerToInsert, 2*time.Minute)
			Expect(err).NotTo(HaveOccurred())

			insertedWorker2, err = database.SaveWorker(workerToInsert2, 2*time.Minute)
			Expect(err).NotTo(HaveOccurred())

			volumeToInsert.WorkerID = insertedWorker.ID
			err = database.InsertVolume(volumeToInsert)
			Expect(err).NotTo(HaveOccurred())
		})

		It("can be retrieved", func() {
			volumes, err := database.GetVolumes()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(volumes)).To(Equal(1))
			actualVolume := volumes[0]
			Expect(actualVolume.WorkerID).To(Equal(volumeToInsert.WorkerID))
			Expect(actualVolume.TTL).To(Equal(volumeToInsert.TTL))
			Expect(actualVolume.ExpiresIn).To(BeNumerically("~", volumeToInsert.TTL, time.Second))
			Expect(actualVolume.Handle).To(Equal(volumeToInsert.Handle))
			Expect(actualVolume.ResourceVersion).To(Equal(volumeToInsert.ResourceVersion))
			Expect(actualVolume.ResourceHash).To(Equal(volumeToInsert.ResourceHash))
			Expect(actualVolume.WorkerName).To(Equal(insertedWorker.Name))
		})

		It("can be reaped", func() {
			volumeToInsert2 := db.Volume{
				WorkerID:        insertedWorker2.ID,
				TTL:             time.Hour,
				Handle:          "some-volume-handle2",
				ResourceVersion: atc.Version{"some": "version"},
				ResourceHash:    "some-hash2",
			}
			err = database.InsertVolume(volumeToInsert2)
			Expect(err).NotTo(HaveOccurred())

			workerToInsert3 := db.WorkerInfo{
				GardenAddr:       "third-garden-address",
				BaggageclaimURL:  "some-baggageclaim-url",
				ActiveContainers: 0,
				ResourceTypes:    []atc.WorkerResourceType{},
				Platform:         "linux",
				Tags:             []string{"vsphere"},
				Name:             "third-worker",
			}
			insertedWorker3, err := database.SaveWorker(workerToInsert3, 2*time.Minute)
			Expect(err).NotTo(HaveOccurred())
			volumeToInsert3 := db.Volume{
				WorkerID:        insertedWorker3.ID,
				TTL:             time.Hour,
				Handle:          "some-volume-handle3",
				ResourceVersion: atc.Version{"some": "version"},
				ResourceHash:    "some-hash3",
			}
			err = database.InsertVolume(volumeToInsert3)
			Expect(err).NotTo(HaveOccurred())

			volumes, err := database.GetVolumes()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(volumes)).To(Equal(3))

			reapedVolume := volumes[0]
			err = database.ReapVolume(reapedVolume.Handle)
			Expect(err).NotTo(HaveOccurred())

			volumes, err = database.GetVolumes()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(volumes)).To(Equal(2))
			Expect(volumes).NotTo(ContainElement(reapedVolume))
		})

		It("can insert the same data twice, without erroring or data duplication", func() {
			err := database.InsertVolume(volumeToInsert)
			Expect(err).NotTo(HaveOccurred())

			volumes, err := database.GetVolumes()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(volumes)).To(Equal(1))
		})

		It("can create the same volume on a different worker", func() {
			volumeToInsert.WorkerID = insertedWorker2.ID
			err := database.InsertVolume(volumeToInsert)
			Expect(err).NotTo(HaveOccurred())

			volumes, err := database.GetVolumes()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(volumes)).To(Equal(2))
		})

		Context("expired volumes", func() {
			BeforeEach(func() {
				volumeToInsert.TTL = -time.Hour
			})

			It("does not return them", func() {
				volumes, err := database.GetVolumes()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(volumes)).To(Equal(0))
			})
		})

		Context("TTL's", func() {
			It("can be retrieved by volume handler", func() {
				actualTTL, err := database.GetVolumeTTL(volumeToInsert.Handle)
				Expect(err).NotTo(HaveOccurred())
				Expect(actualTTL).To(Equal(volumeToInsert.TTL))
			})

			It("can be updated", func() {
				volumes, err := database.GetVolumes()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(volumes)).To(Equal(1))

				err = database.SetVolumeTTL(volumes[0].Handle, -time.Hour)
				Expect(err).NotTo(HaveOccurred())

				volumes, err = database.GetVolumes()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(volumes)).To(Equal(0))
			})

			It("can be updated to zero to mean 'keep around forever'", func() {
				volumes, err := database.GetVolumes()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(volumes)).To(Equal(1))

				err = database.SetVolumeTTL(volumes[0].Handle, 0)
				Expect(err).NotTo(HaveOccurred())

				volumes, err = database.GetVolumes()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(volumes)).To(Equal(1))

				Expect(volumes[0].TTL).To(BeZero())
				Expect(volumes[0].ExpiresIn).To(BeZero())
			})

			Context("when the ttl is set to 0", func() {
				BeforeEach(func() {
					volumeToInsert.TTL = 0
				})

				It("sets the expiration to null", func() {
					volumes, err := database.GetVolumes()
					Expect(err).NotTo(HaveOccurred())
					Expect(len(volumes)).To(Equal(1))
					Expect(volumes[0].TTL).To(Equal(time.Duration(0)))
					Expect(volumes[0].ExpiresIn).To(Equal(time.Duration(0)))
				})
			})
		})
	})
})
