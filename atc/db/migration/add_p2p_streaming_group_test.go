package migration_test

import (
	"github.com/concourse/concourse/v8/atc/db/migration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Add P2P streaming group", func() {
	const preMigrationVersion = 1787865309

	It("upgrades existing workers and supports rollback", func() {
		conn := postgresRunner.OpenDBAtVersion(preMigrationVersion)
		defer conn.Close()

		var columnExists bool
		err := conn.QueryRow(`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'workers'
			AND column_name = 'p2p_streaming_group'
		)`).Scan(&columnExists)
		Expect(err).NotTo(HaveOccurred())
		// The migration must come after the schema deployed before this feature.
		Expect(columnExists).To(BeFalse())

		_, err = conn.Exec("INSERT INTO workers (name) VALUES ('existing-worker')")
		Expect(err).NotTo(HaveOccurred())

		migrator := migration.NewMigrator(conn, nil)
		Expect(migrator.Up(nil, nil)).To(Succeed())

		var group string
		err = conn.QueryRow("SELECT p2p_streaming_group FROM workers WHERE name = 'existing-worker'").Scan(&group)
		Expect(err).NotTo(HaveOccurred())
		Expect(group).To(BeEmpty())

		err = conn.QueryRow("INSERT INTO workers (name) VALUES ('new-worker') RETURNING p2p_streaming_group").Scan(&group)
		Expect(err).NotTo(HaveOccurred())
		Expect(group).To(BeEmpty())

		_, err = conn.Exec("UPDATE workers SET p2p_streaming_group = 'group-a' WHERE name = 'existing-worker'")
		Expect(err).NotTo(HaveOccurred())
		_, err = conn.Exec("UPDATE workers SET p2p_streaming_group = NULL WHERE name = 'existing-worker'")
		Expect(err).To(HaveOccurred())

		Expect(migrator.Migrate(nil, nil, preMigrationVersion)).To(Succeed())
		_, err = conn.Exec("SELECT p2p_streaming_group FROM workers")
		Expect(err).To(HaveOccurred())
		var count int
		Expect(conn.QueryRow("SELECT COUNT(*) FROM workers").Scan(&count)).To(Succeed())
		Expect(count).To(Equal(2))
	})
})
