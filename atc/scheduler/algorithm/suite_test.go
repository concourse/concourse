package algorithm_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"go.opentelemetry.io/otel/exporter/trace/jaeger"

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
		spanSyncer, err := (tracing.Jaeger{
			Endpoint: jaegerURL + "/api/traces",
			Service:  "algorithm_test",
		}).Exporter()
		Expect(err).ToNot(HaveOccurred())

		exporter = spanSyncer.(*jaeger.Exporter)

		tp := tracing.TraceProvider(exporter)
		tracing.ConfigureTraceProvider(tp)
	}

	postgresRunner = postgresrunner.Runner{
		Port: 5433 + GinkgoParallelNode(),
	}

	dbProcess = ifrit.Invoke(postgresRunner)

	postgresRunner.CreateTestDB()
})

var _ = BeforeEach(func() {
	postgresRunner.Truncate()

	dbConn = postgresRunner.OpenConn()

	lockFactory = lock.NewLockFactory(postgresRunner.OpenSingleton(), metric.LogLockAcquired, metric.LogLockReleased)
	teamFactory = db.NewTeamFactory(dbConn, lockFactory)
})

var _ = AfterEach(func() {
	err := dbConn.Close()
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	dbProcess.Signal(os.Interrupt)
	<-dbProcess.Wait()

	if exporter != nil {
		exporter.Flush()
	}
})
