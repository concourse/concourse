package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// fakeRowScanner is a minimal squirrel.RowScanner that returns a fixed bool.
type fakeRowScanner struct {
	val interface{}
	err error
}

func (f *fakeRowScanner) Scan(dest ...interface{}) error {
	if f.err != nil {
		return f.err
	}
	if len(dest) > 0 {
		if d, ok := dest[0].(*bool); ok {
			*d = f.val.(bool)
		}
	}
	return nil
}

var _ sq.RowScanner = &fakeRowScanner{}

func makeWorker(state db.WorkerState) *dbfakes.FakeWorker {
	w := new(dbfakes.FakeWorker)
	w.StateReturns(state)
	return w
}

func dbHealthyStub(_ context.Context, _ string, _ ...any) sq.RowScanner {
	return &fakeRowScanner{val: false} // pg_is_in_recovery = false → writable
}

func dbReadOnlyStub(_ context.Context, _ string, _ ...any) sq.RowScanner {
	return &fakeRowScanner{val: true} // pg_is_in_recovery = true → standby
}

func dbDownStub(_ context.Context, _ string, _ ...any) sq.RowScanner {
	return &fakeRowScanner{err: errors.New("connection refused")}
}

var _ = Describe("Health API", func() {
	var response *http.Response

	JustBeforeEach(func() {
		var err error
		response, err = client.Get(server.URL + "/api/v1/health")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		response.Body.Close()
	})

	Describe("GET /api/v1/health", func() {
		Context("when the database is healthy and workers meet the threshold", func() {
			BeforeEach(func() {
				fakeDbConn.QueryRowContextStub = dbHealthyStub
				dbWorkerFactory.WorkersReturns([]db.Worker{
					makeWorker(db.WorkerStateRunning),
					makeWorker(db.WorkerStateRunning),
				}, nil)
			})

			It("returns 200", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns status ok with healthy subsystems", func() {
				body, _ := io.ReadAll(response.Body)
				var health atc.Health
				Expect(json.Unmarshal(body, &health)).To(Succeed())
				Expect(health.Status).To(Equal(atc.HealthStatusOK))
				Expect(health.Database.Status).To(Equal(atc.HealthStatusHealthy))
				Expect(health.Workers.Status).To(Equal(atc.HealthStatusHealthy))
				Expect(health.Workers.Total).To(Equal(2))
				Expect(health.Workers.Running).To(Equal(2))
			})

			It("includes a recent timestamp", func() {
				body, _ := io.ReadAll(response.Body)
				var health atc.Health
				Expect(json.Unmarshal(body, &health)).To(Succeed())
				Expect(health.Timestamp).To(BeTemporally("~", time.Now().UTC(), 5*time.Second))
			})
		})

		Context("when running workers exactly meet the minimum threshold", func() {
			BeforeEach(func() {
				fakeDbConn.QueryRowContextStub = dbHealthyStub
				// minWorkerCount=1 in suite; 1 running, 1 stalled → exactly meets threshold
				dbWorkerFactory.WorkersReturns([]db.Worker{
					makeWorker(db.WorkerStateRunning),
					makeWorker(db.WorkerStateStalled),
				}, nil)
			})

			It("returns 200", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns status ok", func() {
				body, _ := io.ReadAll(response.Body)
				var health atc.Health
				Expect(json.Unmarshal(body, &health)).To(Succeed())
				Expect(health.Status).To(Equal(atc.HealthStatusOK))
				Expect(health.Workers.Status).To(Equal(atc.HealthStatusHealthy))
				Expect(health.Workers.Running).To(Equal(1))
			})
		})

		Context("when running workers are below the minimum threshold but at least one exists", func() {
			BeforeEach(func() {
				fakeDbConn.QueryRowContextStub = dbHealthyStub
				// minWorkerCount=1, but 0 running → degraded (there are workers, just none running)
				// Actually with total=1 stalled, running=0 < minWorkerCount=1 → degraded path
				// But our logic says running==0 → unhealthy, so let's test with minWorkerCount override
				// The suite hardcodes minWorkerCount=1; to test degraded we'd need minWorkerCount=2.
				// Instead test the failing path: stalled workers, running=0.
				dbWorkerFactory.WorkersReturns([]db.Worker{
					makeWorker(db.WorkerStateStalled),
				}, nil)
			})

			It("returns 503", func() {
				Expect(response.StatusCode).To(Equal(http.StatusServiceUnavailable))
			})

			It("reports workers as unhealthy (no running workers)", func() {
				body, _ := io.ReadAll(response.Body)
				var health atc.Health
				Expect(json.Unmarshal(body, &health)).To(Succeed())
				Expect(health.Status).To(Equal(atc.HealthStatusFailing))
				Expect(health.Workers.Status).To(Equal(atc.HealthStatusUnhealthy))
				Expect(health.Workers.Total).To(Equal(1))
				Expect(health.Workers.Running).To(Equal(0))
			})
		})

		Context("when there are no workers at all", func() {
			BeforeEach(func() {
				fakeDbConn.QueryRowContextStub = dbHealthyStub
				dbWorkerFactory.WorkersReturns([]db.Worker{}, nil)
			})

			It("returns 503", func() {
				Expect(response.StatusCode).To(Equal(http.StatusServiceUnavailable))
			})

			It("reports workers as unhealthy with zero counts", func() {
				body, _ := io.ReadAll(response.Body)
				var health atc.Health
				Expect(json.Unmarshal(body, &health)).To(Succeed())
				Expect(health.Status).To(Equal(atc.HealthStatusFailing))
				Expect(health.Workers.Status).To(Equal(atc.HealthStatusUnhealthy))
				Expect(health.Workers.Total).To(Equal(0))
				Expect(health.Workers.Running).To(Equal(0))
			})
		})

		Context("when the database is unreachable", func() {
			BeforeEach(func() {
				fakeDbConn.QueryRowContextStub = dbDownStub
				dbWorkerFactory.WorkersReturns([]db.Worker{
					makeWorker(db.WorkerStateRunning),
				}, nil)
			})

			It("returns 503", func() {
				Expect(response.StatusCode).To(Equal(http.StatusServiceUnavailable))
			})

			It("reports database as unhealthy and overall status as failing", func() {
				body, _ := io.ReadAll(response.Body)
				var health atc.Health
				Expect(json.Unmarshal(body, &health)).To(Succeed())
				Expect(health.Status).To(Equal(atc.HealthStatusFailing))
				Expect(health.Database.Status).To(Equal(atc.HealthStatusUnhealthy))
				Expect(health.Database.Error).NotTo(BeEmpty())
			})
		})

		Context("when the database is read-only (standby replica)", func() {
			BeforeEach(func() {
				fakeDbConn.QueryRowContextStub = dbReadOnlyStub
				dbWorkerFactory.WorkersReturns([]db.Worker{
					makeWorker(db.WorkerStateRunning),
				}, nil)
			})

			It("returns 503", func() {
				Expect(response.StatusCode).To(Equal(http.StatusServiceUnavailable))
			})

			It("reports database as unhealthy with a read-only error message", func() {
				body, _ := io.ReadAll(response.Body)
				var health atc.Health
				Expect(json.Unmarshal(body, &health)).To(Succeed())
				Expect(health.Status).To(Equal(atc.HealthStatusFailing))
				Expect(health.Database.Status).To(Equal(atc.HealthStatusUnhealthy))
				Expect(health.Database.Error).To(ContainSubstring("read-only"))
			})
		})

		Context("when workers cannot be fetched from the database", func() {
			BeforeEach(func() {
				fakeDbConn.QueryRowContextStub = dbHealthyStub
				dbWorkerFactory.WorkersReturns(nil, errors.New("db error"))
			})

			It("returns 503", func() {
				Expect(response.StatusCode).To(Equal(http.StatusServiceUnavailable))
			})

			It("reports workers as unhealthy", func() {
				body, _ := io.ReadAll(response.Body)
				var health atc.Health
				Expect(json.Unmarshal(body, &health)).To(Succeed())
				Expect(health.Status).To(Equal(atc.HealthStatusFailing))
				Expect(health.Workers.Status).To(Equal(atc.HealthStatusUnhealthy))
			})
		})
	})
})
