package db_test

// import (
// 	. "github.com/onsi/ginkgo"
// 	. "github.com/onsi/gomega"
// )

// create cache volume logic:
//
// 1. open tx
// 2. lookup cache id
//   * if not found, create.
//     * if fails (unique violation; concurrent create), goto 1.
// 2. insert into volumes in 'initializing' state
//   * if fails (fkey violation; preexisting cache id was removed), goto 1.
// 3. commit tx
//
// use cache volume logic:
//
// 1. open tx
// 2. set cache volume state to 'initialized' if 'initializing' or 'initialized'
//    * if fails, return false; must be 'deleting'
// 3. insert into volumes with parent id and parent state
//    * if fails, return false; transitioned to as it was previously 'initialized'
// 4. commit tx
//
// 'get' looks like:
//
// 1. lookup cache volume
//	 * if found, goto 4.
// 2. create cache volume
// 3. initialize cache volume
// 4. use cache volume
//   * if false returned, goto 2.

// var _ = Describe("Database integrity for volume lifecycle", func() {
// 	var dbConn db.Conn

// 	BeforeEach(func() {
// 		postgresRunner.Truncate()

// 		dbConn = db.Wrap(postgresRunner.Open())

// 		// these tests explicitly cover concurrent transactions
// 		dbConn.SetMaxOpenConns(10)
// 	})

// 	AfterEach(func() {
// 		err := dbConn.Close()
// 		Expect(err).NotTo(HaveOccurred())
// 	})

// 	FIt("does not let a cache be deleted while a volume is initializing it", func() {
// 		var workerName string
// 		Expect(dbConn.QueryRow(`
// 			INSERT INTO workers (name, addr)
// 			VALUES ('some-worker', 'bogus')
// 			RETURNING name
// 		`).Scan(&workerName)).To(Succeed())

// 		var workerResourceVersionID int
// 		Expect(dbConn.QueryRow(`
// 			INSERT INTO worker_resource_versions (worker_name, type, image, version)
// 			VALUES ($1, 'some-type', 'some-image', 'some-version')
// 			RETURNING id
// 		`, workerName).Scan(&workerResourceVersionID)).To(Succeed())

// 		var resourceTypeVolumeID int
// 		Expect(dbConn.QueryRow(`
// 			INSERT INTO volumes (worker_name, worker_resource_version_id, handle, state)
// 			VALUES ($1, $2, 'rtv', 'initialized')
// 			RETURNING id
// 		`, workerName, workerResourceVersionID).Scan(&resourceTypeVolumeID)).To(Succeed())

// 		var cacheID int
// 		Expect(dbConn.QueryRow(`
// 			INSERT INTO caches (resource_type_volume_id, source_hash, params_hash, version)
// 			VALUES ($1, 'source-hash', 'params-hash', 'version')
// 			RETURNING id
// 		`, resourceTypeVolumeID).Scan(&cacheID)).To(Succeed())

// 		var cacheVolumeID int
// 		Expect(dbConn.QueryRow(`
// 			INSERT INTO volumes (worker_name, cache_id, handle, state)
// 			VALUES ($1, $2, 'c', 'initializing')
// 			RETURNING id
// 		`, workerName, cacheID).Scan(&cacheVolumeID)).To(Succeed())

// 		_, err := dbConn.Exec(`
// 			DELETE FROM caches WHERE id = $1
// 		`, cacheID)
// 		Expect(err).To(HaveOccurred())
// 		Expect(err.(*pq.Error).Constraint).To(Equal("cannot_invalidate_during_initialization"))
// 	})

// 	PIt("concurrent upsert condition (insert first)", func() {
// 		var workerName string
// 		Expect(dbConn.QueryRow(`
// 			INSERT INTO workers (name, addr)
// 			VALUES ('some-worker', 'bogus')
// 			RETURNING name
// 		`).Scan(&workerName)).To(Succeed())

// 		var workerResourceVersionID int
// 		Expect(dbConn.QueryRow(`
// 			INSERT INTO worker_resource_versions (worker_name, type, image, version)
// 			VALUES ($1, 'some-type', 'some-image', 'some-version')
// 			RETURNING id
// 		`, workerName).Scan(&workerResourceVersionID)).To(Succeed())

// 		var resourceTypeVolumeID int
// 		Expect(dbConn.QueryRow(`
// 			INSERT INTO volumes (worker_name, worker_resource_version_id, handle, state)
// 			VALUES ($1, $2, 'rtv', 'initialized')
// 			RETURNING id
// 		`, workerName, workerResourceVersionID).Scan(&resourceTypeVolumeID)).To(Succeed())

// 		var cacheID int
// 		Expect(dbConn.QueryRow(`
// 			INSERT INTO caches (resource_type_volume_id, source_hash, params_hash, version)
// 			VALUES ($1, 'source-hash', 'params-hash', 'version')
// 			RETURNING id
// 		`, resourceTypeVolumeID).Scan(&cacheID)).To(Succeed())

// 		upsertTx, err := dbConn.Begin()
// 		Expect(err).ToNot(HaveOccurred())

// 		defer upsertTx.Rollback()

// 		gcTx, err := dbConn.Begin()
// 		Expect(err).ToNot(HaveOccurred())

// 		defer gcTx.Rollback()

// 		var cacheVolumeID int

// 		Expect(upsertTx.QueryRow(`
// 			INSERT INTO volumes (worker_name, cache_id, handle, state)
// 			VALUES ($1, $2, 'c', 'initializing')
// 			RETURNING id
// 		`, workerName, cacheID).Scan(&cacheVolumeID)).To(Succeed())

// 		_, err = gcTx.Exec(`
// 			DELETE FROM caches WHERE id = $1
// 		`, cacheID)
// 		Expect(err).To(HaveOccurred())
// 		Expect(err.(*pq.Error).Constraint).To(Equal("cannot_invalidate_during_initialization"))
// 	})

// 	FIt("concurrent upsert condition (delete first)", func() {
// 		var workerName string
// 		Expect(dbConn.QueryRow(`
// 			INSERT INTO workers (name, addr)
// 			VALUES ('some-worker', 'bogus')
// 			RETURNING name
// 		`).Scan(&workerName)).To(Succeed())

// 		var workerResourceVersionID int
// 		Expect(dbConn.QueryRow(`
// 			INSERT INTO worker_resource_versions (worker_name, type, image, version)
// 			VALUES ($1, 'some-type', 'some-image', 'some-version')
// 			RETURNING id
// 		`, workerName).Scan(&workerResourceVersionID)).To(Succeed())

// 		var resourceTypeVolumeID int
// 		Expect(dbConn.QueryRow(`
// 			INSERT INTO volumes (worker_name, worker_resource_version_id, handle, state)
// 			VALUES ($1, $2, 'rtv', 'initialized')
// 			RETURNING id
// 		`, workerName, workerResourceVersionID).Scan(&resourceTypeVolumeID)).To(Succeed())

// 		var cacheID int
// 		Expect(dbConn.QueryRow(`
// 			INSERT INTO caches (resource_type_volume_id, source_hash, params_hash, version)
// 			VALUES ($1, 'source-hash', 'params-hash', 'version')
// 			RETURNING id
// 		`, resourceTypeVolumeID).Scan(&cacheID)).To(Succeed())

// 		upsertTx, err := dbConn.Begin()
// 		Expect(err).ToNot(HaveOccurred())

// 		defer upsertTx.Rollback()

// 		var upsertOriginalCacheID int
// 		Expect(upsertTx.QueryRow(`
// 			SELECT id FROM caches
// 			WHERE source_hash = 'source-hash'
// 			AND params_hash = 'params-hash'
// 			AND version = 'version'
// 		`).Scan(&upsertOriginalCacheID)).To(Succeed())

// 		gcTx, err := dbConn.Begin()
// 		Expect(err).ToNot(HaveOccurred())

// 		defer gcTx.Rollback()

// 		var cacheVolumeID int

// 		_, err = gcTx.Exec(`
// 			DELETE FROM caches WHERE id = $1
// 		`, cacheID)
// 		Expect(err).ToNot(HaveOccurred())
// 		// Expect(err.(*pq.Error).Constraint).To(Equal("cannot_invalidate_during_initialization"))

// 		Expect(gcTx.Commit()).To(Succeed())

// 		// NOTE: this can get a fkey error!
// 		err = upsertTx.QueryRow(`
// 			INSERT INTO volumes (worker_name, cache_id, handle, state)
// 			VALUES ($1, $2, 'c', 'initializing')
// 			RETURNING id
// 		`, workerName, upsertOriginalCacheID).Scan(&cacheVolumeID)
// 		Expect(err).To(HaveOccurred())
// 		Expect(err.(*pq.Error).Constraint).To(Equal("volumes_cache_id_fkey"))

// 		newCacheTx, err := dbConn.Begin()
// 		Expect(err).ToNot(HaveOccurred())

// 		var recoveredCacheID int
// 		Expect(newCacheTx.QueryRow(`
// 			INSERT INTO caches (resource_type_volume_id, source_hash, params_hash, version)
// 			VALUES ($1, 'source-hash', 'params-hash', 'version')
// 			RETURNING id
// 		`, resourceTypeVolumeID).Scan(&recoveredCacheID)).To(Succeed())

// 		Expect(newCacheTx.QueryRow(`
// 			INSERT INTO volumes (worker_name, cache_id, handle, state)
// 			VALUES ($1, $2, 'c', 'initializing')
// 			RETURNING id
// 		`, workerName, recoveredCacheID).Scan(&cacheVolumeID)).To(Succeed())

// 		Expect(newCacheTx.Commit()).To(Succeed())
// 	})

// 	FIt("concurrent insert condition (delete first)", func() {
// 		var workerName string
// 		Expect(dbConn.QueryRow(`
// 			INSERT INTO workers (name, addr)
// 			VALUES ('some-worker', 'bogus')
// 			RETURNING name
// 		`).Scan(&workerName)).To(Succeed())

// 		var workerResourceVersionID int
// 		Expect(dbConn.QueryRow(`
// 			INSERT INTO worker_resource_versions (worker_name, type, image, version)
// 			VALUES ($1, 'some-type', 'some-image', 'some-version')
// 			RETURNING id
// 		`, workerName).Scan(&workerResourceVersionID)).To(Succeed())

// 		var resourceTypeVolumeID int
// 		Expect(dbConn.QueryRow(`
// 			INSERT INTO volumes (worker_name, worker_resource_version_id, handle, state)
// 			VALUES ($1, $2, 'rtv', 'initialized')
// 			RETURNING id
// 		`, workerName, workerResourceVersionID).Scan(&resourceTypeVolumeID)).To(Succeed())

// 		var cacheID int
// 		Expect(dbConn.QueryRow(`
// 			INSERT INTO caches (resource_type_volume_id, source_hash, params_hash, version)
// 			VALUES ($1, 'source-hash', 'params-hash', 'version')
// 			RETURNING id
// 		`, resourceTypeVolumeID).Scan(&cacheID)).To(Succeed())

// 		upsertTx, err := dbConn.Begin()
// 		Expect(err).ToNot(HaveOccurred())

// 		defer upsertTx.Rollback()

// 		var upsertOriginalCacheID int
// 		Expect(upsertTx.QueryRow(`
// 			SELECT id FROM caches
// 			WHERE source_hash = 'source-hash'
// 			AND params_hash = 'params-hash'
// 			AND version = 'version'
// 		`).Scan(&upsertOriginalCacheID)).To(Succeed())

// 		gcTx, err := dbConn.Begin()
// 		Expect(err).ToNot(HaveOccurred())

// 		defer gcTx.Rollback()

// 		var cacheVolumeID int

// 		_, err = gcTx.Exec(`
// 			DELETE FROM caches WHERE id = $1
// 		`, cacheID)
// 		Expect(err).ToNot(HaveOccurred())
// 		// Expect(err.(*pq.Error).Constraint).To(Equal("cannot_invalidate_during_initialization"))

// 		Expect(gcTx.Commit()).To(Succeed())

// 		// NOTE: this can get a fkey error!
// 		err = upsertTx.QueryRow(`
// 			INSERT INTO volumes (worker_name, cache_id, handle, state)
// 			VALUES ($1, $2, 'c', 'initializing')
// 			RETURNING id
// 		`, workerName, upsertOriginalCacheID).Scan(&cacheVolumeID)
// 		Expect(err).To(HaveOccurred())
// 		Expect(err.(*pq.Error).Constraint).To(Equal("volumes_cache_id_fkey"))

// 		newCacheTx1, err := dbConn.Begin()
// 		Expect(err).ToNot(HaveOccurred())

// 		newCacheTx2, err := dbConn.Begin()
// 		Expect(err).ToNot(HaveOccurred())

// 		var recoveredCacheID1 int
// 		Expect(newCacheTx1.QueryRow(`
// 			INSERT INTO caches (resource_type_volume_id, source_hash, params_hash, version)
// 			VALUES ($1, 'source-hash', 'params-hash', 'version')
// 			RETURNING id
// 		`, resourceTypeVolumeID).Scan(&recoveredCacheID1)).To(Succeed())

// 		var recoveredCacheID2 int
// 		Expect(newCacheTx2.QueryRow(`
// 			INSERT INTO caches (resource_type_volume_id, source_hash, params_hash, version)
// 			VALUES ($1, 'source-hash', 'params-hash', 'version')
// 			RETURNING id
// 		`, resourceTypeVolumeID).Scan(&recoveredCacheID2)).To(Succeed())

// 		Expect(newCacheTx1.QueryRow(`
// 			INSERT INTO volumes (worker_name, cache_id, handle, state)
// 			VALUES ($1, $2, 'c1', 'initializing')
// 			RETURNING id
// 		`, workerName, recoveredCacheID1).Scan(&cacheVolumeID)).To(Succeed())

// 		Expect(newCacheTx1.Commit()).To(Succeed())

// 		Expect(newCacheTx2.QueryRow(`
// 			INSERT INTO volumes (worker_name, cache_id, handle, state)
// 			VALUES ($1, $2, 'c2', 'initializing')
// 			RETURNING id
// 		`, workerName, recoveredCacheID2).Scan(&cacheVolumeID)).To(Succeed())

// 		Expect(newCacheTx2.Commit()).To(Succeed())
// 	})
// })
