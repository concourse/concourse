package dbng_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cache", func() {
	var dbConn dbng.Conn
	var tx dbng.Tx

	var cache dbng.Cache

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = dbng.Wrap(postgresRunner.Open())

		tf := dbng.NewTeamFactory(dbConn)
		bf := dbng.NewBuildFactory(dbConn)
		vf := dbng.NewVolumeFactory(dbConn)
		cf := dbng.NewContainerFactory(dbConn)

		team, err := tf.CreateTeam("some-team")
		Expect(err).ToNot(HaveOccurred())

		build, err := bf.CreateOneOffBuild(team)
		Expect(err).ToNot(HaveOccurred())

		worker := &dbng.Worker{
			Name:       "some-worker",
			GardenAddr: "1.2.3.4:7777",
		}

		setupTx, err := dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		err = worker.Create(setupTx)
		Expect(err).ToNot(HaveOccurred())

		wrt := dbng.WorkerResourceType{
			WorkerName: worker.Name,
			Type:       "some-worker-resource-type",
			Image:      "some-worker-resource-image",
			Version:    "some-worker-resource-version",
		}

		_, err = wrt.Create(setupTx)
		Expect(err).ToNot(HaveOccurred())

		Expect(setupTx.Commit()).To(Succeed())

		creatingVolume, err := vf.CreateWorkerResourceTypeVolume(wrt)
		Expect(err).ToNot(HaveOccurred())

		creatingContainer, err := cf.CreateStepContainer(worker, build, "some-plan", dbng.ContainerMetadata{
			Type: "task",
			Name: "some-task",
		})
		Expect(err).ToNot(HaveOccurred())

		setupTx, err = dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		created, err := creatingVolume.Created(setupTx, "some-imported-handle")
		Expect(err).ToNot(HaveOccurred())

		initializing, err := created.Initializing(setupTx)
		Expect(err).ToNot(HaveOccurred())

		initialized, containerVol, err := initializing.Use(setupTx, creatingContainer)
		Expect(err).ToNot(HaveOccurred())

		createdContainerVol, err := containerVol.Created(setupTx, "some-volume-handle")
		Expect(err).ToNot(HaveOccurred())

		createdContainer, err := creatingContainer.Created(setupTx, "some-container-handle", []*dbng.CreatedVolume{createdContainerVol})
		Expect(err).ToNot(HaveOccurred())

		destroyingContainerVol, err := containerVol.Destroying()
		Expect(err).ToNot(HaveOccurred())

		Expect(destroyingContainerVol.Destroy()).To(BeTrue())

		Expect(setupTx.Commit()).To(Succeed())

		tx, err = dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		cache = dbng.Cache{
			ResourceTypeVolume: initialized,
			Source:             atc.Source{"some": "source"},
			Params:             atc.Params{"some": "params"},
			Version:            atc.Version{"some": "version"},
		}
	})

	AfterEach(func() {
		err := tx.Rollback()
		Expect(err).NotTo(HaveOccurred())

		err = dbConn.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	It("can be created and looked up", func() {
		foundID, found, err := cache.Lookup(tx)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeFalse())
		Expect(foundID).To(BeZero())

		createdID, err := cache.Create(tx)
		Expect(err).ToNot(HaveOccurred())
		Expect(createdID).ToNot(BeZero())

		foundID, found, err = cache.Lookup(tx)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(foundID).To(Equal(createdID))
	})

	Context("when it already exists", func() {
		BeforeEach(func() {
			_, err := cache.Create(tx)
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns ErrCacheAlreadyExists", func() {
			id, err := cache.Create(tx)
			Expect(err).To(Equal(dbng.ErrCacheAlreadyExists))
			Expect(id).To(BeZero())
		})
	})

	Context("when the resource type volume starts to be destroyed", func() {
		var destroying *dbng.DestroyingVolume

		BeforeEach(func() {
			var err error
			destroying, err = cache.ResourceTypeVolume.Destroying(tx)
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns ErrCacheResourceTypeVolumeDisappeared", func() {
			id, err := cache.Create(tx)
			Expect(err).To(Equal(dbng.ErrCacheResourceTypeVolumeDisappeared))
			Expect(id).To(BeZero())
		})

		Context("when the resource type volume is destroyed", func() {
			BeforeEach(func() {
				Expect(destroying.Destroy(tx)).To(BeTrue())
			})

			It("returns ErrCacheResourceTypeVolumeDisappeared", func() {
				id, err := cache.Create(tx)
				Expect(err).To(Equal(dbng.ErrCacheResourceTypeVolumeDisappeared))
				Expect(id).To(BeZero())
			})
		})
	})

	// FIt("does not let a cache be deleted while a volume is initializing it", func() {
	// 	var workerName string
	// 	Expect(dbConn.QueryRow(`
	// 		INSERT INTO workers (name, addr)
	// 		VALUES ('some-worker', 'bogus')
	// 		RETURNING name
	// 	`).Scan(&workerName)).To(Succeed())

	// 	var workerResourceVersionID int
	// 	Expect(dbConn.QueryRow(`
	// 		INSERT INTO worker_resource_versions (worker_name, type, image, version)
	// 		VALUES ($1, 'some-type', 'some-image', 'some-version')
	// 		RETURNING id
	// 	`, workerName).Scan(&workerResourceVersionID)).To(Succeed())

	// 	var resourceTypeVolumeID int
	// 	Expect(dbConn.QueryRow(`
	// 		INSERT INTO volumes (worker_name, worker_resource_version_id, handle, state)
	// 		VALUES ($1, $2, 'rtv', 'initialized')
	// 		RETURNING id
	// 	`, workerName, workerResourceVersionID).Scan(&resourceTypeVolumeID)).To(Succeed())

	// 	var cacheID int
	// 	Expect(dbConn.QueryRow(`
	// 		INSERT INTO caches (resource_type_volume_id, source_hash, params_hash, version)
	// 		VALUES ($1, 'source-hash', 'params-hash', 'version')
	// 		RETURNING id
	// 	`, resourceTypeVolumeID).Scan(&cacheID)).To(Succeed())

	// 	var cacheVolumeID int
	// 	Expect(dbConn.QueryRow(`
	// 		INSERT INTO volumes (worker_name, cache_id, handle, state)
	// 		VALUES ($1, $2, 'c', 'initializing')
	// 		RETURNING id
	// 	`, workerName, cacheID).Scan(&cacheVolumeID)).To(Succeed())

	// 	_, err := dbConn.Exec(`
	// 		DELETE FROM caches WHERE id = $1
	// 	`, cacheID)
	// 	Expect(err).To(HaveOccurred())
	// 	Expect(err.(*pq.Error).Constraint).To(Equal("cannot_invalidate_during_initialization"))
	// })

	// PIt("concurrent upsert condition (insert first)", func() {
	// 	var workerName string
	// 	Expect(dbConn.QueryRow(`
	// 		INSERT INTO workers (name, addr)
	// 		VALUES ('some-worker', 'bogus')
	// 		RETURNING name
	// 	`).Scan(&workerName)).To(Succeed())

	// 	var workerResourceVersionID int
	// 	Expect(dbConn.QueryRow(`
	// 		INSERT INTO worker_resource_versions (worker_name, type, image, version)
	// 		VALUES ($1, 'some-type', 'some-image', 'some-version')
	// 		RETURNING id
	// 	`, workerName).Scan(&workerResourceVersionID)).To(Succeed())

	// 	var resourceTypeVolumeID int
	// 	Expect(dbConn.QueryRow(`
	// 		INSERT INTO volumes (worker_name, worker_resource_version_id, handle, state)
	// 		VALUES ($1, $2, 'rtv', 'initialized')
	// 		RETURNING id
	// 	`, workerName, workerResourceVersionID).Scan(&resourceTypeVolumeID)).To(Succeed())

	// 	var cacheID int
	// 	Expect(dbConn.QueryRow(`
	// 		INSERT INTO caches (resource_type_volume_id, source_hash, params_hash, version)
	// 		VALUES ($1, 'source-hash', 'params-hash', 'version')
	// 		RETURNING id
	// 	`, resourceTypeVolumeID).Scan(&cacheID)).To(Succeed())

	// 	upsertTx, err := dbConn.Begin()
	// 	Expect(err).ToNot(HaveOccurred())

	// 	defer upsertTx.Rollback()

	// 	gcTx, err := dbConn.Begin()
	// 	Expect(err).ToNot(HaveOccurred())

	// 	defer gcTx.Rollback()

	// 	var cacheVolumeID int

	// 	Expect(upsertTx.QueryRow(`
	// 		INSERT INTO volumes (worker_name, cache_id, handle, state)
	// 		VALUES ($1, $2, 'c', 'initializing')
	// 		RETURNING id
	// 	`, workerName, cacheID).Scan(&cacheVolumeID)).To(Succeed())

	// 	_, err = gcTx.Exec(`
	// 		DELETE FROM caches WHERE id = $1
	// 	`, cacheID)
	// 	Expect(err).To(HaveOccurred())
	// 	Expect(err.(*pq.Error).Constraint).To(Equal("cannot_invalidate_during_initialization"))
	// })

	// FIt("concurrent upsert condition (delete first)", func() {
	// 	var workerName string
	// 	Expect(dbConn.QueryRow(`
	// 		INSERT INTO workers (name, addr)
	// 		VALUES ('some-worker', 'bogus')
	// 		RETURNING name
	// 	`).Scan(&workerName)).To(Succeed())

	// 	var workerResourceVersionID int
	// 	Expect(dbConn.QueryRow(`
	// 		INSERT INTO worker_resource_versions (worker_name, type, image, version)
	// 		VALUES ($1, 'some-type', 'some-image', 'some-version')
	// 		RETURNING id
	// 	`, workerName).Scan(&workerResourceVersionID)).To(Succeed())

	// 	var resourceTypeVolumeID int
	// 	Expect(dbConn.QueryRow(`
	// 		INSERT INTO volumes (worker_name, worker_resource_version_id, handle, state)
	// 		VALUES ($1, $2, 'rtv', 'initialized')
	// 		RETURNING id
	// 	`, workerName, workerResourceVersionID).Scan(&resourceTypeVolumeID)).To(Succeed())

	// 	var cacheID int
	// 	Expect(dbConn.QueryRow(`
	// 		INSERT INTO caches (resource_type_volume_id, source_hash, params_hash, version)
	// 		VALUES ($1, 'source-hash', 'params-hash', 'version')
	// 		RETURNING id
	// 	`, resourceTypeVolumeID).Scan(&cacheID)).To(Succeed())

	// 	upsertTx, err := dbConn.Begin()
	// 	Expect(err).ToNot(HaveOccurred())

	// 	defer upsertTx.Rollback()

	// 	var upsertOriginalCacheID int
	// 	Expect(upsertTx.QueryRow(`
	// 		SELECT id FROM caches
	// 		WHERE source_hash = 'source-hash'
	// 		AND params_hash = 'params-hash'
	// 		AND version = 'version'
	// 	`).Scan(&upsertOriginalCacheID)).To(Succeed())

	// 	gcTx, err := dbConn.Begin()
	// 	Expect(err).ToNot(HaveOccurred())

	// 	defer gcTx.Rollback()

	// 	var cacheVolumeID int

	// 	_, err = gcTx.Exec(`
	// 		DELETE FROM caches WHERE id = $1
	// 	`, cacheID)
	// 	Expect(err).ToNot(HaveOccurred())
	// 	// Expect(err.(*pq.Error).Constraint).To(Equal("cannot_invalidate_during_initialization"))

	// 	Expect(gcTx.Commit()).To(Succeed())

	// 	// NOTE: this can get a fkey error!
	// 	err = upsertTx.QueryRow(`
	// 		INSERT INTO volumes (worker_name, cache_id, handle, state)
	// 		VALUES ($1, $2, 'c', 'initializing')
	// 		RETURNING id
	// 	`, workerName, upsertOriginalCacheID).Scan(&cacheVolumeID)
	// 	Expect(err).To(HaveOccurred())
	// 	Expect(err.(*pq.Error).Constraint).To(Equal("volumes_cache_id_fkey"))

	// 	newCacheTx, err := dbConn.Begin()
	// 	Expect(err).ToNot(HaveOccurred())

	// 	var recoveredCacheID int
	// 	Expect(newCacheTx.QueryRow(`
	// 		INSERT INTO caches (resource_type_volume_id, source_hash, params_hash, version)
	// 		VALUES ($1, 'source-hash', 'params-hash', 'version')
	// 		RETURNING id
	// 	`, resourceTypeVolumeID).Scan(&recoveredCacheID)).To(Succeed())

	// 	Expect(newCacheTx.QueryRow(`
	// 		INSERT INTO volumes (worker_name, cache_id, handle, state)
	// 		VALUES ($1, $2, 'c', 'initializing')
	// 		RETURNING id
	// 	`, workerName, recoveredCacheID).Scan(&cacheVolumeID)).To(Succeed())

	// 	Expect(newCacheTx.Commit()).To(Succeed())
	// })

	// FIt("concurrent insert condition (delete first)", func() {
	// 	var workerName string
	// 	Expect(dbConn.QueryRow(`
	// 		INSERT INTO workers (name, addr)
	// 		VALUES ('some-worker', 'bogus')
	// 		RETURNING name
	// 	`).Scan(&workerName)).To(Succeed())

	// 	var workerResourceVersionID int
	// 	Expect(dbConn.QueryRow(`
	// 		INSERT INTO worker_resource_versions (worker_name, type, image, version)
	// 		VALUES ($1, 'some-type', 'some-image', 'some-version')
	// 		RETURNING id
	// 	`, workerName).Scan(&workerResourceVersionID)).To(Succeed())

	// 	var resourceTypeVolumeID int
	// 	Expect(dbConn.QueryRow(`
	// 		INSERT INTO volumes (worker_name, worker_resource_version_id, handle, state)
	// 		VALUES ($1, $2, 'rtv', 'initialized')
	// 		RETURNING id
	// 	`, workerName, workerResourceVersionID).Scan(&resourceTypeVolumeID)).To(Succeed())

	// 	var cacheID int
	// 	Expect(dbConn.QueryRow(`
	// 		INSERT INTO caches (resource_type_volume_id, source_hash, params_hash, version)
	// 		VALUES ($1, 'source-hash', 'params-hash', 'version')
	// 		RETURNING id
	// 	`, resourceTypeVolumeID).Scan(&cacheID)).To(Succeed())

	// 	upsertTx, err := dbConn.Begin()
	// 	Expect(err).ToNot(HaveOccurred())

	// 	defer upsertTx.Rollback()

	// 	var upsertOriginalCacheID int
	// 	Expect(upsertTx.QueryRow(`
	// 		SELECT id FROM caches
	// 		WHERE source_hash = 'source-hash'
	// 		AND params_hash = 'params-hash'
	// 		AND version = 'version'
	// 	`).Scan(&upsertOriginalCacheID)).To(Succeed())

	// 	gcTx, err := dbConn.Begin()
	// 	Expect(err).ToNot(HaveOccurred())

	// 	defer gcTx.Rollback()

	// 	var cacheVolumeID int

	// 	_, err = gcTx.Exec(`
	// 		DELETE FROM caches WHERE id = $1
	// 	`, cacheID)
	// 	Expect(err).ToNot(HaveOccurred())
	// 	// Expect(err.(*pq.Error).Constraint).To(Equal("cannot_invalidate_during_initialization"))

	// 	Expect(gcTx.Commit()).To(Succeed())

	// 	// NOTE: this can get a fkey error!
	// 	err = upsertTx.QueryRow(`
	// 		INSERT INTO volumes (worker_name, cache_id, handle, state)
	// 		VALUES ($1, $2, 'c', 'initializing')
	// 		RETURNING id
	// 	`, workerName, upsertOriginalCacheID).Scan(&cacheVolumeID)
	// 	Expect(err).To(HaveOccurred())
	// 	Expect(err.(*pq.Error).Constraint).To(Equal("volumes_cache_id_fkey"))

	// 	newCacheTx1, err := dbConn.Begin()
	// 	Expect(err).ToNot(HaveOccurred())

	// 	newCacheTx2, err := dbConn.Begin()
	// 	Expect(err).ToNot(HaveOccurred())

	// 	var recoveredCacheID1 int
	// 	Expect(newCacheTx1.QueryRow(`
	// 		INSERT INTO caches (resource_type_volume_id, source_hash, params_hash, version)
	// 		VALUES ($1, 'source-hash', 'params-hash', 'version')
	// 		RETURNING id
	// 	`, resourceTypeVolumeID).Scan(&recoveredCacheID1)).To(Succeed())

	// 	var recoveredCacheID2 int
	// 	Expect(newCacheTx2.QueryRow(`
	// 		INSERT INTO caches (resource_type_volume_id, source_hash, params_hash, version)
	// 		VALUES ($1, 'source-hash', 'params-hash', 'version')
	// 		RETURNING id
	// 	`, resourceTypeVolumeID).Scan(&recoveredCacheID2)).To(Succeed())

	// 	Expect(newCacheTx1.QueryRow(`
	// 		INSERT INTO volumes (worker_name, cache_id, handle, state)
	// 		VALUES ($1, $2, 'c1', 'initializing')
	// 		RETURNING id
	// 	`, workerName, recoveredCacheID1).Scan(&cacheVolumeID)).To(Succeed())

	// 	Expect(newCacheTx1.Commit()).To(Succeed())

	// 	Expect(newCacheTx2.QueryRow(`
	// 		INSERT INTO volumes (worker_name, cache_id, handle, state)
	// 		VALUES ($1, $2, 'c2', 'initializing')
	// 		RETURNING id
	// 	`, workerName, recoveredCacheID2).Scan(&cacheVolumeID)).To(Succeed())

	// 	Expect(newCacheTx2.Commit()).To(Succeed())
	// })
})
