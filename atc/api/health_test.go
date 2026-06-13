package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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

func makeComponent(name string, interval time.Duration, lastRan time.Time, paused bool) *dbfakes.FakeComponent {
	c := new(dbfakes.FakeComponent)
	c.NameReturns(name)
	c.IntervalReturns(interval)
	c.LastRanReturns(lastRan)
	c.PausedReturns(paused)
	return c
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

		Context("when running workers are above zero but below the minimum threshold", func() {
			var customServer *httptest.Server

			BeforeEach(func() {
				fakeDbConn.QueryRowContextStub = dbHealthyStub
				// 2 running workers, minWorkerCount=3 → below threshold → degraded
				dbWorkerFactory.WorkersReturns([]db.Worker{
					makeWorker(db.WorkerStateRunning),
					makeWorker(db.WorkerStateRunning),
					makeWorker(db.WorkerStateStalled),
				}, nil)
				customServer = buildTestServer(3)
			})

			AfterEach(func() {
				customServer.Close()
			})

			It("returns 200 (degraded is not failing)", func() {
				response, err := client.Get(customServer.URL + "/api/v1/health")
				Expect(err).NotTo(HaveOccurred())
				defer response.Body.Close()
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns status degraded", func() {
				response, err := client.Get(customServer.URL + "/api/v1/health")
				Expect(err).NotTo(HaveOccurred())
				defer response.Body.Close()

				body, _ := io.ReadAll(response.Body)
				var health atc.Health
				Expect(json.Unmarshal(body, &health)).To(Succeed())
				Expect(health.Status).To(Equal(atc.HealthStatusDegraded))
				Expect(health.Workers.Status).To(Equal(atc.HealthStatusDegraded))
				Expect(health.Workers.Running).To(Equal(2))
				Expect(health.Workers.Total).To(Equal(3))
			})
		})

		Context("when running workers are below the minimum threshold but at least one exists", func() {
			BeforeEach(func() {
				fakeDbConn.QueryRowContextStub = dbHealthyStub
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

		Context("when a runtime component is stale", func() {
			BeforeEach(func() {
				fakeDbConn.QueryRowContextStub = dbHealthyStub
				dbWorkerFactory.WorkersReturns([]db.Worker{
					makeWorker(db.WorkerStateRunning),
				}, nil)
				// scheduler last ran 10 minutes ago, interval is 1 minute → 10x > 2x multiplier → stale
				dbComponentFactory.AllReturns([]db.Component{
					makeComponent(atc.ComponentScheduler, time.Minute, time.Now().Add(-10*time.Minute), false),
					makeComponent(atc.ComponentBuildTracker, time.Minute, time.Now().Add(-30*time.Second), false),
				}, nil)
			})

			It("returns 200 (degraded is not failing)", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("reports overall status as degraded", func() {
				body, _ := io.ReadAll(response.Body)
				var health atc.Health
				Expect(json.Unmarshal(body, &health)).To(Succeed())
				Expect(health.Status).To(Equal(atc.HealthStatusDegraded))
			})

			It("marks the scheduler component as unhealthy", func() {
				body, _ := io.ReadAll(response.Body)
				var health atc.Health
				Expect(json.Unmarshal(body, &health)).To(Succeed())

				var scheduler *atc.ComponentHealth
				for i := range health.Components {
					if health.Components[i].Name == atc.ComponentScheduler {
						scheduler = &health.Components[i]
					}
				}
				Expect(scheduler).NotTo(BeNil())
				Expect(scheduler.Status).To(Equal(atc.HealthStatusUnhealthy))
			})

			It("marks the healthy tracker component as healthy", func() {
				body, _ := io.ReadAll(response.Body)
				var health atc.Health
				Expect(json.Unmarshal(body, &health)).To(Succeed())

				var tracker *atc.ComponentHealth
				for i := range health.Components {
					if health.Components[i].Name == atc.ComponentBuildTracker {
						tracker = &health.Components[i]
					}
				}
				Expect(tracker).NotTo(BeNil())
				Expect(tracker.Status).To(Equal(atc.HealthStatusHealthy))
			})
		})

		Context("when only a GC component is stale", func() {
			BeforeEach(func() {
				fakeDbConn.QueryRowContextStub = dbHealthyStub
				dbWorkerFactory.WorkersReturns([]db.Worker{
					makeWorker(db.WorkerStateRunning),
				}, nil)
				// collector_volumes last ran 1 hour ago, interval is 1 minute → stale, but GC only
				dbComponentFactory.AllReturns([]db.Component{
					makeComponent(atc.ComponentScheduler, time.Minute, time.Now().Add(-30*time.Second), false),
					makeComponent(atc.ComponentCollectorVolumes, time.Minute, time.Now().Add(-time.Hour), false),
				}, nil)
			})

			It("returns 200", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("reports overall status as ok", func() {
				body, _ := io.ReadAll(response.Body)
				var health atc.Health
				Expect(json.Unmarshal(body, &health)).To(Succeed())
				Expect(health.Status).To(Equal(atc.HealthStatusOK))
			})

			It("marks the GC component as unhealthy in the JSON", func() {
				body, _ := io.ReadAll(response.Body)
				var health atc.Health
				Expect(json.Unmarshal(body, &health)).To(Succeed())

				var collector *atc.ComponentHealth
				for i := range health.Components {
					if health.Components[i].Name == atc.ComponentCollectorVolumes {
						collector = &health.Components[i]
					}
				}
				Expect(collector).NotTo(BeNil())
				Expect(collector.Status).To(Equal(atc.HealthStatusUnhealthy))
			})
		})

		Context("when a component is paused", func() {
			BeforeEach(func() {
				fakeDbConn.QueryRowContextStub = dbHealthyStub
				dbWorkerFactory.WorkersReturns([]db.Worker{
					makeWorker(db.WorkerStateRunning),
				}, nil)
				// scheduler is paused and hasn't run — should not be considered stale
				dbComponentFactory.AllReturns([]db.Component{
					makeComponent(atc.ComponentScheduler, time.Minute, time.Now().Add(-time.Hour), true),
				}, nil)
			})

			It("returns 200 with status ok", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
				body, _ := io.ReadAll(response.Body)
				var health atc.Health
				Expect(json.Unmarshal(body, &health)).To(Succeed())
				Expect(health.Status).To(Equal(atc.HealthStatusOK))
			})

			It("reports the component as paused and healthy", func() {
				body, _ := io.ReadAll(response.Body)
				var health atc.Health
				Expect(json.Unmarshal(body, &health)).To(Succeed())

				Expect(health.Components).To(HaveLen(1))
				Expect(health.Components[0].Paused).To(BeTrue())
				Expect(health.Components[0].Status).To(Equal(atc.HealthStatusHealthy))
			})
		})

		Context("when component factory returns an error", func() {
			BeforeEach(func() {
				fakeDbConn.QueryRowContextStub = dbHealthyStub
				dbWorkerFactory.WorkersReturns([]db.Worker{
					makeWorker(db.WorkerStateRunning),
				}, nil)
				dbComponentFactory.AllReturns(nil, errors.New("db error"))
			})

			It("returns 200 with status ok and empty components list", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
				body, _ := io.ReadAll(response.Body)
				var health atc.Health
				Expect(json.Unmarshal(body, &health)).To(Succeed())
				Expect(health.Status).To(Equal(atc.HealthStatusOK))
				Expect(health.Components).To(BeEmpty())
			})
		})
	})
})
