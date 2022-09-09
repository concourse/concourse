package migration_test

import (
	"database/sql"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Add Cert Cache Table", func() {

	const postMigrationVersion = 1557237784
	const preMigrationVersion = 1556724983

	var (
		db *sql.DB
	)

	Context("Up", func() {
		It("successfully creates table cert_cache", func() {
			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			writeAutoCert(db, "magic-domain", "iamcert", "iamnonce")
			cert, err := readAutoCert(db, "magic-domain")
			Expect(err).ToNot(HaveOccurred())
			Expect(cert).To(Equal("iamcert"))
			db.Close()
		})
	})

	Context("Down", func() {
		It("successfully drops table cert_cache", func() {
			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			writeAutoCert(db, "magic-domain", "iamcert", "iamnonce")
			db.Close()

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			_, err := readAutoCert(db, "magic-domain")
			Expect(err).To(HaveOccurred())
			db.Close()
		})
	})

})

func readAutoCert(dbConn *sql.DB, domain string) (string, error) {
	var cert []byte
	err := dbConn.QueryRow("SELECT cert FROM cert_cache WHERE domain = $1", domain).Scan(&cert)
	return string(cert), err
}

func writeAutoCert(dbConn *sql.DB, domain, cert, nonce string) {
	dbConn.Exec("INSERT INTO cert_cache(domain, cert, nonce) VALUES($1, $2, $3)", domain, cert, nonce)
}
