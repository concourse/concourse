package algorithm_test

import (
	"context"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"go.opentelemetry.io/otel/exporters/trace/jaeger"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/postgresrunner"
	"github.com/concourse/concourse/tracing"
)

var (
	postgresRunner postgresrunner.Runner
	dbProcess      ifrit.Process

	lockFactory lock.LockFactory
	teamFactory db.TeamFactory

	dbConn db.Conn

	exporter *jaeger.Exporter
)

func TestAlgorithm(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Algorithm Suite")
}

var _ = BeforeSuite(func() {
	jaegerURL := os.Getenv("JAEGER_URL")

	if jaegerURL != "" {
		c := tracing.Config{
			Jaeger: tracing.Jaeger{
				Endpoint: jaegerURL + "/api/traces",
				Service:  "algorithm_test",
			},
		}

		err := c.Prepare()
		Expect(err).ToNot(HaveOccurred())
	}

	postgresrunner.InitializeRunnerForGinkgo(&postgresRunner, &dbProcess)
})

var _ = BeforeEach(func() {
	postgresRunner.CreateTestDBFromTemplate()

	dbConn = postgresRunner.OpenConn()

	lockFactory = lock.NewLockFactory(postgresRunner.OpenSingleton(), metric.LogLockAcquired, metric.LogLockReleased)
	teamFactory = db.NewTeamFactory(dbConn, lockFactory)
})

var _ = AfterEach(func() {
	err := dbConn.Close()
	Expect(err).NotTo(HaveOccurred())

	postgresRunner.DropTestDB()
})

var _ = AfterSuite(func() {
	postgresrunner.FinalizeRunnerForGinkgo(&postgresRunner, &dbProcess)

	if exporter != nil {
		exporter.Shutdown(context.Background())
	}
})
