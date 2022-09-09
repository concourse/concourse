package migration_test

import (
	"testing/fstest"

	"github.com/concourse/concourse/atc/db/migration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var basicSQLMigration = []byte(`
	-- create a table
	CREATE TABLE some_table;
`)

var basicSQLDownMigration = []byte(`
	-- create a table
	DROP TABLE some_table;
`)

var _ = Describe("Parser", func() {
	var (
		parser *migration.Parser
	)

	BeforeEach(func() {
		parser = migration.NewParser(fstest.MapFS{
			"1000_some_migration.up.sql": &fstest.MapFile{
				Data: basicSQLMigration,
			},
			"1000_some_migration.down.sql": &fstest.MapFile{
				Data: basicSQLDownMigration,
			},
			"2000_some_go_migration.up.go": &fstest.MapFile{
				Data: []byte(`
func (m *Migrator) Up_2000() {}
`),
			},
			"2000_some_go_migration.down.go": &fstest.MapFile{
				Data: []byte(`
func (m *Migrator) Down_2000() {}
`),
			},
		})
	})

	It("parses the direction of the migration from the file name", func() {
		downMigration, err := parser.ParseFileToMigration("2000_some_go_migration.down.go")
		Expect(err).ToNot(HaveOccurred())
		Expect(downMigration.Direction).To(Equal("down"))

		upMigration, err := parser.ParseFileToMigration("1000_some_migration.up.sql")
		Expect(err).ToNot(HaveOccurred())
		Expect(upMigration.Direction).To(Equal("up"))
	})

	It("parses the strategy of the migration from the file", func() {
		downMigration, err := parser.ParseFileToMigration("2000_some_go_migration.down.go")
		Expect(err).ToNot(HaveOccurred())
		Expect(downMigration.Strategy).To(Equal(migration.GoMigration))

		upMigration, err := parser.ParseFileToMigration("1000_some_migration.up.sql")
		Expect(err).ToNot(HaveOccurred())
		Expect(upMigration.Strategy).To(Equal(migration.SQLMigration))
	})

	Context("SQL migrations", func() {
		It("parses the migration into statements", func() {
			migration, err := parser.ParseFileToMigration("1000_some_migration.up.sql")
			Expect(err).ToNot(HaveOccurred())
			Expect(migration.Statements).To(Equal(string(basicSQLMigration)))

			migration, err = parser.ParseFileToMigration("1000_some_migration.down.sql")
			Expect(err).ToNot(HaveOccurred())
			Expect(migration.Statements).To(Equal(string(basicSQLDownMigration)))
		})
	})

	Context("Go migrations", func() {
		It("returns the name of the migration function to run", func() {
			migration, err := parser.ParseFileToMigration("2000_some_go_migration.up.go")
			Expect(err).ToNot(HaveOccurred())
			Expect(migration.Name).To(Equal("Up_2000"))
		})
	})
})
