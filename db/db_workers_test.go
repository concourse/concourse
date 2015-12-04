package db_test

import (
	"database/sql"
	"time"

	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

var _ = Describe("Keeping track of workers", func() {
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
})
