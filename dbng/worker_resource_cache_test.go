package dbng_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WorkerResourceCache", func() {
	var workerResourceCache dbng.WorkerResourceCache

	Describe("FindOrCreate", func() {
		BeforeEach(func() {
			resourceCache, err := resourceCacheFactory.FindOrCreateResourceCache(
				logger,
				dbng.ForResource{defaultResource.ID},
				"some-base-resource-type",
				atc.Version{"some": "version"},
				atc.Source{"some": "source"},
				atc.Params{},
				defaultPipeline.ID(),
				atc.ResourceTypes{},
			)
			Expect(err).NotTo(HaveOccurred())

			workerResourceCache = dbng.WorkerResourceCache{
				ResourceCache: resourceCache,
				WorkerName:    defaultWorker.Name(),
			}
		})

		Context("when there are no existing worker resource caches", func() {
			It("creates worker resource cache", func() {
				tx, err := dbConn.Begin()
				Expect(err).NotTo(HaveOccurred())
				defer tx.Rollback()

				usedWorkerResourceCache, err := workerResourceCache.FindOrCreate(tx)
				Expect(err).NotTo(HaveOccurred())

				Expect(usedWorkerResourceCache.ID).To(Equal(1))
			})
		})

		Context("when there is existing worker resource caches", func() {
			var createdWorkerResourceCache *dbng.UsedWorkerResourceCache

			BeforeEach(func() {
				var err error
				tx, err := dbConn.Begin()
				Expect(err).NotTo(HaveOccurred())

				createdWorkerResourceCache, err = workerResourceCache.FindOrCreate(tx)
				Expect(err).NotTo(HaveOccurred())

				Expect(tx.Commit()).To(Succeed())
			})

			It("finds worker resource cache", func() {
				tx, err := dbConn.Begin()
				Expect(err).NotTo(HaveOccurred())
				defer tx.Rollback()

				usedWorkerResourceCache, err := workerResourceCache.FindOrCreate(tx)
				Expect(err).NotTo(HaveOccurred())

				Expect(usedWorkerResourceCache.ID).To(Equal(1))
			})
		})
	})
})
