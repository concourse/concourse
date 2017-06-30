package migration_test

import (
	"os"
	"time"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/postgresrunner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"

	"reflect"
	"runtime"
	"testing"

	"database/sql"

	"github.com/concourse/atc/db/migration"
	"github.com/concourse/atc/db/migrations"
)

func TestMigration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Migration Suite")
}

var postgresRunner postgresrunner.Runner
var dbProcess ifrit.Process

var _ = BeforeSuite(func() {
	postgresRunner = postgresrunner.Runner{
		Port: 5433 + GinkgoParallelNode(),
	}
	dbProcess = ifrit.Invoke(postgresRunner)

})

var _ = BeforeEach(func() {
	postgresRunner.CreateTestDB()
})

var _ = AfterEach(func() {
	postgresRunner.DropTestDB()
})

var _ = AfterSuite(func() {
	dbProcess.Signal(os.Interrupt)
	Eventually(dbProcess.Wait(), 10*time.Second).Should(Receive())
})

func openDBConnPreMigration(migrator migration.Migrator) (*sql.DB, error) {
	return migration.Open(
		"postgres",
		postgresRunner.DataSourceName(),
		migrationsBefore(migrator),
	)
}

func openDBConnPostMigration(migrator migration.Migrator) (*sql.DB, error) {
	migrationsToRun := append(migrationsBefore(migrator), migrator)

	return migration.Open(
		"postgres",
		postgresRunner.DataSourceName(),
		migrationsToRun,
	)
}

func migrationsBefore(migrator migration.Migrator) []migration.Migrator {
	migratorName := migrationFunctionName(migrator)
	migrationsBefore := []migration.Migrator{}

	for _, m := range migrations.New(db.NewNoEncryption()) {
		if migratorName == migrationFunctionName(m) {
			break
		}

		migrationsBefore = append(migrationsBefore, m)
	}

	return migrationsBefore
}

func migrationFunctionName(f interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
}
