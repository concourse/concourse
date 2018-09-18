package db_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WorkerResourceCache", func() {
	var workerResourceCache db.WorkerResourceCache

	Describe("FindOrCreate", func() {
		BeforeEach(func() {
			build, err := defaultTeam.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			resourceCache, err := resourceCacheFactory.FindOrCreateResourceCache(
				logger,
				db.ForBuild(build.ID()),
				"some-base-resource-type",
				atc.Version{"some": "version"},
				atc.Source{"some": "source"},
				atc.Params{},
				creds.VersionedResourceTypes{},
			)
			Expect(err).ToNot(HaveOccurred())

			workerResourceCache = db.WorkerResourceCache{
				ResourceCache: resourceCache,
				WorkerName:    defaultWorker.Name(),
			}
		})

		Context("when there are no existing worker resource caches", func() {
			It("creates worker resource cache", func() {
				tx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())
				defer db.Rollback(tx)

				usedWorkerResourceCache, err := workerResourceCache.FindOrCreate(tx)
				Expect(err).ToNot(HaveOccurred())

				Expect(usedWorkerResourceCache.ID).To(Equal(1))
			})
		})

		Context("when there is existing worker resource caches", func() {
			BeforeEach(func() {
				var err error
				tx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				_, err = workerResourceCache.FindOrCreate(tx)
				Expect(err).ToNot(HaveOccurred())

				Expect(tx.Commit()).To(Succeed())
			})

			It("finds worker resource cache", func() {
				tx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())
				defer db.Rollback(tx)

				usedWorkerResourceCache, err := workerResourceCache.FindOrCreate(tx)
				Expect(err).ToNot(HaveOccurred())

				Expect(usedWorkerResourceCache.ID).To(Equal(1))
			})
		})
	})

	Describe("Find", func() {
		var foundWRC *db.UsedWorkerResourceCache
		var found bool
		var findErr error

		BeforeEach(func() {
			build, err := defaultTeam.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			resourceCache, err := resourceCacheFactory.FindOrCreateResourceCache(
				logger,
				db.ForBuild(build.ID()),
				"some-base-resource-type",
				atc.Version{"some": "version"},
				atc.Source{"some": "source"},
				atc.Params{},
				creds.VersionedResourceTypes{},
			)
			Expect(err).ToNot(HaveOccurred())

			workerResourceCache = db.WorkerResourceCache{
				ResourceCache: resourceCache,
				WorkerName:    defaultWorker.Name(),
			}
		})

		JustBeforeEach(func() {
			tx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())
			defer db.Rollback(tx)

			foundWRC, found, findErr = workerResourceCache.Find(tx)
		})

		Context("when there are no existing worker resource caches", func() {
			It("returns false and no error", func() {
				Expect(findErr).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(foundWRC).To(BeNil())
			})
		})

		Context("when the base resource type does not exist on the worker", func() {
			BeforeEach(func() {
				tx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				defer db.Rollback(tx)

				_, err = db.BaseResourceType{
					Name: "some-bogus-resource-type",
				}.FindOrCreate(tx)
				Expect(err).ToNot(HaveOccurred())

				err = tx.Commit()
				Expect(err).ToNot(HaveOccurred())

				build, err := defaultTeam.CreateOneOffBuild()
				Expect(err).ToNot(HaveOccurred())

				resourceCache, err := resourceCacheFactory.FindOrCreateResourceCache(
					logger,
					db.ForBuild(build.ID()),
					"some-bogus-resource-type",
					atc.Version{"some": "version"},
					atc.Source{"some": "source"},
					atc.Params{},
					creds.VersionedResourceTypes{},
				)
				Expect(err).ToNot(HaveOccurred())

				workerResourceCache.ResourceCache = resourceCache
			})

			It("returns false and no error", func() {
				Expect(found).To(BeFalse())
				Expect(foundWRC).To(BeNil())
			})
		})

		Context("when there is existing worker resource caches", func() {
			var createdWorkerResourceCache *db.UsedWorkerResourceCache

			BeforeEach(func() {
				var err error
				tx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				createdWorkerResourceCache, err = workerResourceCache.FindOrCreate(tx)
				Expect(err).ToNot(HaveOccurred())

				Expect(tx.Commit()).To(Succeed())
			})

			It("finds worker resource cache", func() {
				Expect(findErr).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(foundWRC.ID).To(Equal(createdWorkerResourceCache.ID))
			})
		})
	})
})
