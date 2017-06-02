package migrations_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc/db/migration"

	"github.com/concourse/atc/db/migrations"
	"github.com/concourse/atc/db/migrations/migrationsfakes"
)

var _ = Describe("Logging migration progress", func() {
	var logger *lagertest.TestLogger

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("migrations")
	})

	Describe("Translogrifier", func() {
		It("calls the correct number of migratior functions", func() {
			oldMigrations := []migration.Migrator{
				func(otx migration.LimitedTx) error {
					otx.Exec(`SELECT 1`)
					return nil
				},
				func(otx migration.LimitedTx) error {
					otx.Exec(`SELECT 2`)
					return nil
				},
			}

			newMigrations := migrations.Translogrifier(logger, oldMigrations)
			Expect(newMigrations).To(HaveLen(2))
		})

		It("logs the progress and duration", func() {
			oldMigrations := []migration.Migrator{
				func(otx migration.LimitedTx) error {
					otx.Exec(`SELECT 1`)
					return nil
				},
				func(otx migration.LimitedTx) error {
					otx.Exec(`SELECT 2`)
					return nil
				},
			}

			newMigrations := migrations.Translogrifier(logger, oldMigrations)
			Expect(newMigrations).To(HaveLen(2))

			tx := new(migrationsfakes.FakeLimitedTx)

			newMigrations[0](tx)
			Expect(logger).To(gbytes.Say("starting-migration"))
			Expect(logger).To(gbytes.Say("finishing-migration"))
			Expect(logger).To(gbytes.Say("duration"))

			newMigrations[1](tx)
			Expect(logger).To(gbytes.Say("starting-migration"))
			Expect(logger).To(gbytes.Say("finishing-migration"))
			Expect(logger).To(gbytes.Say("duration"))
		})
	})

	Describe("WithLogger", func() {
		It("calls the original migration", func() {
			tx := new(migrationsfakes.FakeLimitedTx)

			originalMigration := func(otx migration.LimitedTx) error {
				_, err := otx.Exec(`SELECT 1`)
				if err != nil {
					return err
				}

				return nil
			}

			newMigration := migrations.WithLogger(logger, originalMigration)
			newMigration(tx)

			Expect(tx.ExecCallCount()).To(Equal(1))
			statement, _ := tx.ExecArgsForCall(0)
			Expect(statement).To(Equal("SELECT 1"))
		})

		It("bubbles errors from the original migrations back up", func() {
			tx := new(migrationsfakes.FakeLimitedTx)

			originalMigration := func(otx migration.LimitedTx) error {
				return errors.New("disaster!")
			}

			newMigration := migrations.WithLogger(logger, originalMigration)
			err := newMigration(tx)

			Expect(err).To(HaveOccurred())
		})

		It("logs before and after the migration", func() {
			tx := new(migrationsfakes.FakeLimitedTx)

			originalMigration := func(otx migration.LimitedTx) error {
				_, err := otx.Exec(`SELECT 1`)
				if err != nil {
					return err
				}

				return nil
			}

			newMigration := migrations.WithLogger(logger, originalMigration)
			newMigration(tx)

			Expect(logger).To(gbytes.Say("starting-migration"))
			Expect(logger).To(gbytes.Say("finishing-migration"))

			Expect(logger.Logs()[0].LogLevel).To(Equal(lager.INFO))
			Expect(logger.Logs()[1].LogLevel).To(Equal(lager.INFO))
		})

		It("logs the time that the migration took to apply", func() {
			tx := new(migrationsfakes.FakeLimitedTx)

			originalMigration := func(otx migration.LimitedTx) error {
				time.Sleep(1 * time.Millisecond)
				return nil
			}

			newMigration := migrations.WithLogger(logger, originalMigration)
			newMigration(tx)

			Expect(logger).To(gbytes.Say("finishing-migration"))
			Expect(logger).To(gbytes.Say("duration"))

			// should "accurately" measure duration
			Expect(logger).NotTo(gbytes.Say("ns"))
		})

		It("puts the name of the migration in the log", func() {
			tx := new(migrationsfakes.FakeLimitedTx)

			newMigration := migrations.WithLogger(logger, HasAName)
			newMigration(tx)

			Expect(logger).To(gbytes.Say(`"HasAName"`))
		})
	})
})

func HasAName(tx migration.LimitedTx) error {
	return nil
}
