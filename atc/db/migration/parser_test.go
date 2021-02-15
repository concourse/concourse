package migration_test

import (
	"github.com/concourse/concourse/atc/db/migration"
	"github.com/concourse/concourse/atc/db/migration/migrationfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var basicSQLMigration = []byte(`BEGIN;
	-- create a table
	CREATE TABLE some_table;
COMMIT;
`)

var _ = Describe("Parser", func() {
	var (
		parser  *migration.Parser
		bindata *migrationfakes.FakeBindata
	)

	BeforeEach(func() {
		bindata = new(migrationfakes.FakeBindata)
		bindata.AssetReturns([]byte{}, nil)

		parser = migration.NewParser(bindata)
	})

	It("parses the direction of the migration from the file name", func() {
		downMigration, err := parser.ParseFileToMigration("2000_some_migration.down.go")
		Expect(err).ToNot(HaveOccurred())
		Expect(downMigration.Direction).To(Equal("down"))

		upMigration, err := parser.ParseFileToMigration("1000_some_migration.up.sql")
		Expect(err).ToNot(HaveOccurred())
		Expect(upMigration.Direction).To(Equal("up"))
	})

	It("parses the strategy of the migration from the file", func() {
		downMigration, err := parser.ParseFileToMigration("2000_some_migration.down.go")
		Expect(err).ToNot(HaveOccurred())
		Expect(downMigration.Strategy).To(Equal(migration.GoMigration))

		bindata.AssetReturns(basicSQLMigration, nil)
		upMigration, err := parser.ParseFileToMigration("1000_some_migration.up.sql")
		Expect(err).ToNot(HaveOccurred())
		Expect(upMigration.Strategy).To(Equal(migration.SQLMigration))
	})

	Context("SQL migrations", func() {
		It("parses the migration into statements", func() {
			bindata.AssetReturns(basicSQLMigration, nil)
			migration, err := parser.ParseFileToMigration("1234_create_and_alter_table.up.sql")
			Expect(err).ToNot(HaveOccurred())
			Expect(migration.Statements).To(Equal(string(basicSQLMigration)))
		})
	})

	Context("Go migrations", func() {
		It("returns the name of the migration function to run", func() {
			bindata.AssetReturns([]byte(`
				func (m *Migrator) Up_2000() {}
			`), nil)

			migration, err := parser.ParseFileToMigration("2000_some_go_migration.up.go")
			Expect(err).ToNot(HaveOccurred())
			Expect(migration.Name).To(Equal("Up_2000"))
		})
	})
})
