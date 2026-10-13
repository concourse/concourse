package db_test

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/v8/atc/db"
)

const failoverTestDriverName = "failover-test-pgx"

var _ = Describe("Failover Resilience", func() {
	var failoverDB *sql.DB

	BeforeEach(func() {
		dsn := postgresRunner.DataSourceName()
		db.SetupConnectionRetryingDriver("pgx", dsn, failoverTestDriverName)

		var err error
		failoverDB, err = sql.Open(failoverTestDriverName, dsn)
		Expect(err).NotTo(HaveOccurred())
		failoverDB.SetMaxOpenConns(3)
		failoverDB.SetMaxIdleConns(1)
	})

	AfterEach(func() {
		dbConn.Exec("ALTER SYSTEM RESET default_transaction_read_only")
		dbConn.Exec("SELECT pg_reload_conf()")
		failoverDB.Close()
	})

	Describe("read_only_sql_transaction (SQLSTATE 25006)", func() {
		It("is converted to driver.ErrBadConn so database/sql discards the stale connection", func() {
			_, err := failoverDB.Exec("SELECT 1")
			Expect(err).NotTo(HaveOccurred())

			_, err = dbConn.Exec("ALTER SYSTEM SET default_transaction_read_only = on")
			Expect(err).NotTo(HaveOccurred())
			_, err = dbConn.Exec("SELECT pg_reload_conf()")
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				_, writeErr := failoverDB.Exec("CREATE TABLE IF NOT EXISTS failover_probe (id int)")
				return writeErr
			}, 5*time.Second, 50*time.Millisecond).Should(Satisfy(func(err error) bool {
				return errors.Is(err, driver.ErrBadConn)
			}), "expected driver.ErrBadConn (connection discarded), not a pg error — the fix is not active")
		})

		It("allows writes to succeed once the database becomes writable again", func() {
			_, err := dbConn.Exec("ALTER SYSTEM SET default_transaction_read_only = on")
			Expect(err).NotTo(HaveOccurred())
			_, err = dbConn.Exec("SELECT pg_reload_conf()")
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				_, e := failoverDB.Exec("CREATE TABLE IF NOT EXISTS failover_probe (id int)")
				return e
			}, 5*time.Second, 50*time.Millisecond).Should(HaveOccurred())

			_, err = dbConn.Exec("ALTER SYSTEM RESET default_transaction_read_only")
			Expect(err).NotTo(HaveOccurred())
			_, err = dbConn.Exec("SELECT pg_reload_conf()")
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				_, e := failoverDB.Exec("CREATE TABLE IF NOT EXISTS failover_probe (id int)")
				return e
			}, 5*time.Second, 50*time.Millisecond).Should(Succeed())
		})
	})
})
