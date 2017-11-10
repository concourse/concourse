package migration_test

import (
	"database/sql"
	"io/ioutil"
	"strings"

	"github.com/concourse/atc/db/migration"
	"github.com/mattes/migrate/database/postgres"
	bindata "github.com/mattes/migrate/source/go-bindata"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Migration", func() {
	var db *sql.DB
	var err error

	BeforeEach(func() {
	})

	AfterEach(func() {
	})

	It("Fails if trying to upgrade prior to migration_version 189", func() {
		db, err = sql.Open("postgres", postgresRunner.DataSourceName())
		Expect(err).NotTo(HaveOccurred())
		defer db.Close()

		tx, err := db.Begin()
		Expect(err).NotTo(HaveOccurred())

		_, err = tx.Exec(`CREATE TABLE migration_version(version int)`)
		Expect(err).NotTo(HaveOccurred())

		_, err = tx.Exec(`INSERT INTO migration_version(version) VALUES(188)`)
		Expect(err).NotTo(HaveOccurred())

		err = tx.Commit()
		Expect(err).NotTo(HaveOccurred())

		_, err = migration.Open("postgres", postgresRunner.DataSourceName())

		Expect(err.Error()).To(Equal("Cannot upgrade from concourse version < 3.6.0 (db version: 189), current db version: 188"))

		_, err = db.Exec("SELECT version FROM migration_version")
		Expect(err).NotTo(HaveOccurred())
	})

	It("Forces mattes/migrate to a known first version if migration_version is 189", func() {
		db, err = sql.Open("postgres", postgresRunner.DataSourceName())
		Expect(err).NotTo(HaveOccurred())
		defer db.Close()

		tx, err := db.Begin()
		Expect(err).NotTo(HaveOccurred())

		_, err = tx.Exec(`CREATE TABLE migration_version(version int)`)
		Expect(err).NotTo(HaveOccurred())

		_, err = tx.Exec(`INSERT INTO migration_version(version) VALUES(189)`)
		Expect(err).NotTo(HaveOccurred())

		migrations, err := ioutil.ReadFile("migrations/1510262030_initial_schema.up.sql")
		Expect(err).NotTo(HaveOccurred())

		for _, migration := range strings.Split(string(migrations), ";") {
			_, err = tx.Exec(migration)
			Expect(err).NotTo(HaveOccurred())
		}

		err = tx.Commit()
		Expect(err).NotTo(HaveOccurred())

		dbConn, err := migration.Open("postgres", postgresRunner.DataSourceName())
		Expect(err).NotTo(HaveOccurred())
		defer dbConn.Close()

		var dbVersion int
		err = dbConn.QueryRow("SELECT version FROM schema_migrations LIMIT 1").Scan(&dbVersion)
		Expect(err).NotTo(HaveOccurred())
		Expect(dbVersion > 0).To(BeTrue())

		_, err = dbConn.Exec("SELECT version FROM migration_version")
		Expect(err).To(HaveOccurred())

		_, err = dbConn.Exec("INSERT INTO teams(name) VALUES ('test-team')")
		Expect(err).NotTo(HaveOccurred())
	})

	It("Delegate migration work to mattes/migrate if migration_version table does not exist", func() {
		dbConn, err := migration.Open("postgres", postgresRunner.DataSourceName())
		Expect(err).NotTo(HaveOccurred())
		defer dbConn.Close()

		var dbVersion int
		err = dbConn.QueryRow("SELECT version FROM schema_migrations LIMIT 1").Scan(&dbVersion)
		Expect(err).NotTo(HaveOccurred())
		Expect(dbVersion > 0).To(BeTrue())

		_, err = dbConn.Exec("SELECT version FROM migration_version")
		Expect(err).To(HaveOccurred())

		_, err = dbConn.Exec("INSERT INTO teams(name) VALUES ('test-team')")
		Expect(err).NotTo(HaveOccurred())
	})

	It("Doesn't fail if mattes/migrate.up() is a no-op", func() {
		db, err = sql.Open("postgres", postgresRunner.DataSourceName())
		Expect(err).NotTo(HaveOccurred())
		defer db.Close()

		tx, err := db.Begin()
		Expect(err).NotTo(HaveOccurred())

		_, err = tx.Exec(`CREATE TABLE schema_migrations(version bigint, dirty boolean)`)
		Expect(err).NotTo(HaveOccurred())

		_, err = tx.Exec(`INSERT INTO schema_migrations(version, dirty) VALUES(1510262030, false)`)
		Expect(err).NotTo(HaveOccurred())

		migrations, err := ioutil.ReadFile("migrations/1510262030_initial_schema.up.sql")
		Expect(err).NotTo(HaveOccurred())

		for _, migration := range strings.Split(string(migrations), ";") {
			_, err = tx.Exec(migration)
			Expect(err).NotTo(HaveOccurred())
		}

		err = tx.Commit()
		Expect(err).NotTo(HaveOccurred())

		d, err := postgres.WithInstance(db, &postgres.Config{})
		Expect(err).NotTo(HaveOccurred())

		s, err := bindata.WithInstance(bindata.Resource(
			[]string{"1510262030_initial_schema.up.sql"},
			func(name string) ([]byte, error) {
				return migration.Asset(name)
			}),
		)

		dbConn, err := migration.OpenWithMigrateDrivers(db, "go-bindata", s, "postgres", d)
		Expect(err).NotTo(HaveOccurred())
		defer dbConn.Close()

		var dbVersion int
		err = dbConn.QueryRow("SELECT version FROM schema_migrations LIMIT 1").Scan(&dbVersion)
		Expect(err).NotTo(HaveOccurred())
		Expect(dbVersion > 0).To(BeTrue())

		_, err = dbConn.Exec("SELECT version FROM migration_version")
		Expect(err).To(HaveOccurred())

		_, err = dbConn.Exec("INSERT INTO teams(name) VALUES ('test-team')")
		Expect(err).NotTo(HaveOccurred())
	})
})
