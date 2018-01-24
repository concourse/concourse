package migration_test

import (
	"io/ioutil"
	"strings"

	"github.com/concourse/atc/db/migration"
	"github.com/concourse/atc/db/migration/migrationfakes"
	"github.com/mattes/migrate/database"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Driver", func() {
	var (
		driver         database.Driver
		fakedriver     *migrationfakes.FakeDriver
		fakemigrations *migrationfakes.FakeMigrations
	)

	BeforeEach(func() {
		fakedriver = new(migrationfakes.FakeDriver)
		fakemigrations = new(migrationfakes.FakeMigrations)

		driver = migration.NewDriverForMigrations(fakedriver, fakemigrations)
	})

	Context("Run", func() {

		Context("golang", func() {

			It("fails if migration does not contain function", func() {

				contents := `package migrations`
				reader := strings.NewReader(contents)

				err := driver.Run(reader)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("No migration found"))
			})

			It("fails if migration does not contain function matching up/down pattern", func() {

				contents := `package migrations

func (self *migrations) Sideways_1234567890() error {
	return nil
}
`
				reader := strings.NewReader(contents)

				err := driver.Run(reader)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("No migration found"))
			})

			It("parses the migration name from the reader contents", func() {

				contents := `package migrations

func (self *migrations) Up_1234567890() error {
	return nil
}
`
				reader := strings.NewReader(contents)

				err := driver.Run(reader)

				Expect(err).NotTo(HaveOccurred())
				Expect(fakemigrations.RunArgsForCall(0)).To(Equal("Up_1234567890"))
			})
		})

		Context("psql", func() {
			It("delegates to the embedded postgres driver", func() {

				contents := "CREATE TABLE blah(id SERIAL)"
				reader := strings.NewReader(contents)

				err := driver.Run(reader)

				Expect(err).NotTo(HaveOccurred())

				calledReader := fakedriver.RunArgsForCall(0)
				calledContents, err := ioutil.ReadAll(calledReader)

				Expect(err).NotTo(HaveOccurred())
				Expect(string(calledContents)).To(Equal(contents))
			})
		})
	})
})
