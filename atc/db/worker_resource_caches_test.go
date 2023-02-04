package db_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbtest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("WorkerResourceCaches", func() {
	var resourceCache db.ResourceCache
	var build db.Build
	var scenario *dbtest.Scenario
	var usedBaseResourceTypeOnWorker0, usedBaseResourceTypeOnWorker1, usedBaseResourceTypeOnWorker2 *db.UsedWorkerBaseResourceType

	BeforeEach(func() {
		scenario = dbtest.Setup(
			builder.WithTeam("some-team"),
			builder.WithBaseWorker(), // worker0
			builder.WithBaseWorker(), // worker1
			builder.WithBaseWorker(), // worker2
		)

		var err error
		build, err = scenario.Team.CreateOneOffBuild()
		Expect(err).ToNot(HaveOccurred())

		resourceTypeCache, err := resourceCacheFactory.FindOrCreateResourceCache(
			db.ForBuild(build.ID()),
			dbtest.BaseResourceType,
			atc.Version{"some-type": "version"},
			atc.Source{
				"some-type": "source",
			},
			nil,
			nil,
		)

		resourceCache, err = resourceCacheFactory.FindOrCreateResourceCache(
			db.ForBuild(build.ID()),
			"some-type",
			atc.Version{"some": "version"},
			atc.Source{
				"some": "source",
			},
			atc.Params{"some": "params"},
			resourceTypeCache,
		)
		Expect(err).ToNot(HaveOccurred())

		var found bool
		usedBaseResourceTypeOnWorker0, found, err = db.WorkerBaseResourceType{
			Name:       resourceCache.BaseResourceType().Name,
			WorkerName: scenario.Workers[0].Name(),
		}.Find(dbConn)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		usedBaseResourceTypeOnWorker1, found, err = db.WorkerBaseResourceType{
			Name:       resourceCache.BaseResourceType().Name,
			WorkerName: scenario.Workers[1].Name(),
		}.Find(dbConn)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		usedBaseResourceTypeOnWorker2, found, err = db.WorkerBaseResourceType{
			Name:       resourceCache.BaseResourceType().Name,
			WorkerName: scenario.Workers[2].Name(),
		}.Find(dbConn)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
	})

	Context("FindOrCreate()", func() {
		var (
			usedWorkerResourceCache *db.UsedWorkerResourceCache
			valid                   bool
			findErr                 error
		)

		Context("Create a worker resource cache on worker0 with it's own base resource type", func() {
			BeforeEach(func() {
				tx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())
				usedWorkerResourceCache, valid, findErr = db.WorkerResourceCache{
					WorkerName:    scenario.Workers[0].Name(),
					ResourceCache: resourceCache,
				}.FindOrCreate(tx, usedBaseResourceTypeOnWorker0.ID)
				tx.Commit()
			})

			It("should create a cache", func() {
				Expect(findErr).ToNot(HaveOccurred())
				Expect(valid).To(BeTrue())
				Expect(usedWorkerResourceCache).ToNot(BeNil())
			})

			Context("Create a worker resource cache again on worker0 again with it's own base resource type", func() {
				var uwrc *db.UsedWorkerResourceCache
				BeforeEach(func() {
					tx, err := dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())
					uwrc, valid, findErr = db.WorkerResourceCache{
						WorkerName:    scenario.Workers[0].Name(),
						ResourceCache: resourceCache,
					}.FindOrCreate(tx, usedBaseResourceTypeOnWorker0.ID)
					tx.Commit()
				})

				It("should find a cache", func() {
					Expect(findErr).ToNot(HaveOccurred())
					Expect(valid).To(BeTrue())
					Expect(uwrc).ToNot(BeNil())
					Expect(*uwrc).To(Equal(*usedWorkerResourceCache))
				})
			})

			Context("Create a worker resource cache again on worker0 with worker1's base resource type", func() {
				var uwrc *db.UsedWorkerResourceCache
				BeforeEach(func() {
					tx, err := dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())
					uwrc, valid, findErr = db.WorkerResourceCache{
						WorkerName:    scenario.Workers[0].Name(),
						ResourceCache: resourceCache,
					}.FindOrCreate(tx, usedBaseResourceTypeOnWorker1.ID)
					tx.Commit()
				})

				It("should not create a cache, but find the existing cache", func() {
					Expect(findErr).ToNot(HaveOccurred())
					Expect(valid).To(BeFalse()) // valid is false as this is not the cache to create
					Expect(uwrc).ToNot(BeNil())
					Expect(*uwrc).To(Equal(*usedWorkerResourceCache))
				})
			})

			Context("Create a worker resource cache on worker1 with worker0's base base resource type", func() {
				var uwrcOnWorker1 *db.UsedWorkerResourceCache
				BeforeEach(func() {
					tx, err := dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())
					uwrcOnWorker1, valid, findErr = db.WorkerResourceCache{
						WorkerName:    scenario.Workers[1].Name(),
						ResourceCache: resourceCache,
					}.FindOrCreate(tx, usedBaseResourceTypeOnWorker0.ID)
					tx.Commit()
				})

				It("should create a cache", func() {
					Expect(findErr).ToNot(HaveOccurred())
					Expect(valid).To(BeTrue())
					Expect(uwrcOnWorker1).ToNot(BeNil())
					Expect(*uwrcOnWorker1).ToNot(Equal(*usedWorkerResourceCache))
					Expect(uwrcOnWorker1.WorkerBaseResourceTypeID).To(Equal(usedBaseResourceTypeOnWorker0.ID))
				})

				Context("Prune worker0", func() {
					BeforeEach(func() {
						err := scenario.Workers[0].Land()
						Expect(err).ToNot(HaveOccurred())
						err = scenario.Workers[0].Prune()
						Expect(err).ToNot(HaveOccurred())
					})

					Context("Invalidated cache", func() {
						var (
							buildStartTime          time.Time
							uwrcOnWorker1AfterPrune *db.UsedWorkerResourceCache
							found                   bool
							err                     error
						)
						JustBeforeEach(func() {
							uwrcOnWorker1AfterPrune, found, err = db.WorkerResourceCache{
								WorkerName:    scenario.Workers[1].Name(),
								ResourceCache: resourceCache,
							}.Find(dbConn, buildStartTime)
						})

						Context("when build started before cache invalidated", func() {
							BeforeEach(func() {
								buildStartTime = time.Now().Add(-100 * time.Second)
							})
							It("should still find an invalidated cache on worker1", func() {
								Expect(err).ToNot(HaveOccurred())
								Expect(found).To(BeTrue())
								Expect(uwrcOnWorker1AfterPrune).ToNot(BeNil())
								Expect(uwrcOnWorker1AfterPrune.ID).To(Equal(uwrcOnWorker1.ID))
								Expect(uwrcOnWorker1AfterPrune.WorkerBaseResourceTypeID).To(BeZero())
							})
						})

						Context("when build started after cache invalidated", func() {
							BeforeEach(func() {
								buildStartTime = time.Now().Add(100 * time.Second)
							})
							It("should not find an invalidated cache on worker1", func() {
								Expect(err).ToNot(HaveOccurred())
								Expect(found).To(BeFalse())
								Expect(uwrcOnWorker1AfterPrune).To(BeNil())
							})
						})
					})

					Context("Create a worker resource cache on worker1 with worker2's base base resource type", func() {
						var newUwrcOnWorker1 *db.UsedWorkerResourceCache
						BeforeEach(func() {
							tx, err := dbConn.Begin()
							Expect(err).ToNot(HaveOccurred())
							newUwrcOnWorker1, valid, findErr = db.WorkerResourceCache{
								WorkerName:    scenario.Workers[1].Name(),
								ResourceCache: resourceCache,
							}.FindOrCreate(tx, usedBaseResourceTypeOnWorker2.ID)
							tx.Commit()
						})

						It("should create a cache", func() {
							Expect(findErr).ToNot(HaveOccurred())
							Expect(valid).To(BeTrue())
							Expect(newUwrcOnWorker1).ToNot(BeNil())
							Expect(*newUwrcOnWorker1).ToNot(Equal(*usedWorkerResourceCache))
							Expect(newUwrcOnWorker1.WorkerBaseResourceTypeID).To(Equal(usedBaseResourceTypeOnWorker2.ID))
						})

						It("should invalidated cache still be there", func() {
							uwrc, found, err := db.WorkerResourceCache{}.FindByID(dbConn, uwrcOnWorker1.ID)
							Expect(err).ToNot(HaveOccurred())
							Expect(found).To(BeTrue())
							Expect(uwrc).ToNot(BeNil())
							Expect(uwrc.WorkerBaseResourceTypeID).To(BeZero())
						})
					})
				})
			})
		})
	})
})
