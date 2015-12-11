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
		var insertedVolume db.Volume

		BeforeEach(func() {
			insertedVolume = db.Volume{
				WorkerName:      "some-worker",
				TTL:             time.Hour,
				Handle:          "some-volume-handle",
				ResourceVersion: atc.Version{"some": "version"},
				ResourceHash:    "some-hash",
			}
		})

		JustBeforeEach(func() {
			err := database.InsertVolume(insertedVolume)
			Expect(err).NotTo(HaveOccurred())
		})

		It("can be retrieved", func() {
			volumes, err := database.GetVolumes()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(volumes)).To(Equal(1))
			actualVolume := volumes[0]
			Expect(actualVolume.WorkerName).To(Equal(insertedVolume.WorkerName))
			Expect(actualVolume.TTL).To(Equal(insertedVolume.TTL))
			Expect(actualVolume.ExpiresIn).To(BeNumerically("~", insertedVolume.TTL, time.Second))
			Expect(actualVolume.Handle).To(Equal(insertedVolume.Handle))
			Expect(actualVolume.ResourceVersion).To(Equal(insertedVolume.ResourceVersion))
			Expect(actualVolume.ResourceHash).To(Equal(insertedVolume.ResourceHash))
		})

		It("can be reaped", func() {
			insertedVolume2 := db.Volume{
				WorkerName:      "some-worker2",
				TTL:             time.Hour,
				Handle:          "some-volume-handle2",
				ResourceVersion: atc.Version{"some": "version"},
				ResourceHash:    "some-hash2",
			}
			insertedVolume3 := db.Volume{
				WorkerName:      "some-worker3",
				TTL:             time.Hour,
				Handle:          "some-volume-handle3",
				ResourceVersion: atc.Version{"some": "version"},
				ResourceHash:    "some-hash3",
			}
			err := database.InsertVolume(insertedVolume2)
			Expect(err).NotTo(HaveOccurred())
			err = database.InsertVolume(insertedVolume3)
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
			err := database.InsertVolume(insertedVolume)
			Expect(err).NotTo(HaveOccurred())

			volumes, err := database.GetVolumes()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(volumes)).To(Equal(1))
		})

		It("can create the same volume on a different worker", func() {
			insertedVolume.WorkerName = "some-other-worker"
			err := database.InsertVolume(insertedVolume)
			Expect(err).NotTo(HaveOccurred())

			volumes, err := database.GetVolumes()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(volumes)).To(Equal(2))
		})

		Context("expired volumes", func() {
			BeforeEach(func() {
				insertedVolume.TTL = -time.Hour
			})

			It("does not return them", func() {
				volumes, err := database.GetVolumes()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(volumes)).To(Equal(0))
			})
		})

		Context("TTL's", func() {
			It("can be retrieved by volume handler", func() {
				actualTTL, err := database.GetVolumeTTL(insertedVolume.Handle)
				Expect(err).NotTo(HaveOccurred())
				Expect(actualTTL).To(Equal(insertedVolume.TTL))
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

			Context("when the ttl is set to 0", func() {
				BeforeEach(func() {
					insertedVolume.TTL = 0
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
