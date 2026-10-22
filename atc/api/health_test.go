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
	"github.com/concourse/concourse/v8/atc"
	"github.com/concourse/concourse/v8/atc/api/healthserver"
	"github.com/concourse/concourse/v8/atc/db"
	"github.com/concourse/concourse/v8/atc/db/dbfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// fakeRowScanner is a minimal squirrel.RowScanner that returns a fixed bool.
type fakeRowScanner struct {
	val any
	err error
}

func (f *fakeRowScanner) Scan(dest ...any) error {
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

func makeNamedWorker(name string, state db.WorkerState) *dbfakes.FakeWorker {
	w := new(dbfakes.FakeWorker)
	w.NameReturns(name)
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

// workerStatus is a string alias matching WorkerHealth.Status — keeps test assertions readable.
const (
	workerStatusHealthy   = string(atc.HealthStatusHealthy)
	workerStatusDegraded  = string(atc.HealthStatusDegraded)
	workerStatusUnhealthy = string(atc.HealthStatusUnhealthy)
)

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
				Expect(health.Workers.Status).To(Equal(workerStatusHealthy))
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
				Expect(health.Workers.Status).To(Equal(workerStatusHealthy))
				Expect(health.Workers.Running).To(Equal(1))
			})
		})

		Context("when running workers are above zero but below the minimum threshold", func() {
			var customServer *httptest.Server

			BeforeEach(func() {
				fakeDbConn.QueryRowContextStub = dbHealthyStub
				// 2 running workers, minWorkerCount=3 → below threshold → degraded
				dbWorkerFactory.WorkersReturns([]db.Worker{
					makeNamedWorker("worker-01", db.WorkerStateRunning),
					makeNamedWorker("worker-02", db.WorkerStateRunning),
					makeNamedWorker("worker-03", db.WorkerStateStalled),
				}, nil)
				customServer = newApiTestServer(withWorkerCount(3))
			})

			AfterEach(func() {
				customServer.Close()
			})

			It("returns 200 (degraded is not failing)", func() {
				resp, err := client.Get(customServer.URL + "/api/v1/health")
				Expect(err).NotTo(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns status degraded with unhealthy workers listed", func() {
				resp, err := client.Get(customServer.URL + "/api/v1/health")
				Expect(err).NotTo(HaveOccurred())
				defer resp.Body.Close()

				body, _ := io.ReadAll(resp.Body)
				var health atc.Health
				Expect(json.Unmarshal(body, &health)).To(Succeed())
				Expect(health.Status).To(Equal(atc.HealthStatusDegraded))
				Expect(health.Workers.Status).To(Equal(workerStatusDegraded))
				Expect(health.Workers.Running).To(Equal(2))
				Expect(health.Workers.Total).To(Equal(3))
				Expect(health.Workers.UnhealthyWorkers).To(ConsistOf("worker-03"))
			})
		})

		Context("when there are no running workers but some are registered", func() {
			BeforeEach(func() {
				fakeDbConn.QueryRowContextStub = dbHealthyStub
				dbWorkerFactory.WorkersReturns([]db.Worker{
					makeNamedWorker("worker-01", db.WorkerStateStalled),
				}, nil)
			})

			It("returns 503", func() {
				Expect(response.StatusCode).To(Equal(http.StatusServiceUnavailable))
			})

			It("reports workers as unhealthy with the stalled worker listed", func() {
				body, _ := io.ReadAll(response.Body)
				var health atc.Health
				Expect(json.Unmarshal(body, &health)).To(Succeed())
				Expect(health.Status).To(Equal(atc.HealthStatusFailing))
				Expect(health.Workers.Status).To(Equal(workerStatusUnhealthy))
				Expect(health.Workers.Total).To(Equal(1))
				Expect(health.Workers.Running).To(Equal(0))
				Expect(health.Workers.UnhealthyWorkers).To(ConsistOf("worker-01"))
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
				Expect(health.Workers.Status).To(Equal(workerStatusUnhealthy))
				Expect(health.Workers.Total).To(Equal(0))
				Expect(health.Workers.Running).To(Equal(0))
				Expect(health.Workers.UnhealthyWorkers).To(BeEmpty())
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
				Expect(health.Workers.Status).To(Equal(workerStatusUnhealthy))
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

// makeDetailedWorker builds a FakeWorker with platform, team, and tags set in
// addition to state. Used by tests that exercise the by_platform/by_team/by_tag
// breakdown fields.
func makeDetailedWorker(state db.WorkerState, platform, team string, tags []string) *dbfakes.FakeWorker {
	w := new(dbfakes.FakeWorker)
	w.StateReturns(state)
	w.PlatformReturns(platform)
	w.TeamNameReturns(team)
	w.TagsReturns(tags)
	return w
}

var _ = Describe("Health API — worker breakdown", func() {
	var response *http.Response

	AfterEach(func() {
		response.Body.Close()
	})

	Context("when workers have platform, team, and tag details", func() {
		BeforeEach(func() {
			fakeDbConn.QueryRowContextStub = dbHealthyStub
			dbComponentFactory.AllReturns(nil, nil)
			dbWorkerFactory.WorkersReturns([]db.Worker{
				// linux / team infra / tags: gpu, spot
				makeDetailedWorker(db.WorkerStateRunning, "linux", "infra", []string{"gpu", "spot"}),
				// linux / global (no team) / tag: spot
				makeDetailedWorker(db.WorkerStateRunning, "linux", "", []string{"spot"}),
				// windows / team infra / no tags
				makeDetailedWorker(db.WorkerStateStalled, "windows", "infra", nil),
			}, nil)
		})

		JustBeforeEach(func() {
			var err error
			response, err = client.Get(server.URL + "/api/v1/health")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 200 ok — threshold met even though one worker is stalled", func() {
			// minWorkerCount=1 (suite default); 2 running workers exceed it.
			// A stalled worker appears in unhealthy_workers but does not
			// degrade the overall status unless the running count drops below
			// the threshold.
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			body, _ := io.ReadAll(response.Body)
			var health atc.Health
			Expect(json.Unmarshal(body, &health)).To(Succeed())
			Expect(health.Status).To(Equal(atc.HealthStatusOK))
			Expect(health.Workers.Status).To(Equal(workerStatusHealthy))
			Expect(health.Workers.UnhealthyWorkers).To(HaveLen(1))
		})

		It("includes correct by_platform totals and running counts", func() {
			body, _ := io.ReadAll(response.Body)
			var health atc.Health
			Expect(json.Unmarshal(body, &health)).To(Succeed())

			Expect(health.Workers.ByPlatform).To(HaveKey("linux"))
			Expect(health.Workers.ByPlatform["linux"].Total).To(Equal(2))
			Expect(health.Workers.ByPlatform["linux"].Running).To(Equal(2))

			Expect(health.Workers.ByPlatform).To(HaveKey("windows"))
			Expect(health.Workers.ByPlatform["windows"].Total).To(Equal(1))
			Expect(health.Workers.ByPlatform["windows"].Running).To(Equal(0))
		})

		It("includes correct by_team totals with empty key for unscoped workers", func() {
			body, _ := io.ReadAll(response.Body)
			var health atc.Health
			Expect(json.Unmarshal(body, &health)).To(Succeed())

			Expect(health.Workers.ByTeam).To(HaveKey(""))
			Expect(health.Workers.ByTeam[""]).To(Equal(atc.WorkerBreakdown{Total: 1, Running: 1}))

			Expect(health.Workers.ByTeam).To(HaveKey("infra"))
			Expect(health.Workers.ByTeam["infra"].Total).To(Equal(2))
			Expect(health.Workers.ByTeam["infra"].Running).To(Equal(1)) // only running linux worker
		})

		It("counts a multi-tagged worker in every matching tag bucket", func() {
			body, _ := io.ReadAll(response.Body)
			var health atc.Health
			Expect(json.Unmarshal(body, &health)).To(Succeed())

			// "spot" appears in both workers (linux/infra + linux/global)
			Expect(health.Workers.ByTag).To(HaveKey("spot"))
			Expect(health.Workers.ByTag["spot"].Total).To(Equal(2))
			Expect(health.Workers.ByTag["spot"].Running).To(Equal(2))

			// "gpu" appears only in the first worker
			Expect(health.Workers.ByTag).To(HaveKey("gpu"))
			Expect(health.Workers.ByTag["gpu"].Total).To(Equal(1))
			Expect(health.Workers.ByTag["gpu"].Running).To(Equal(1))
		})
	})

	Context("with a per-platform threshold that is met", func() {
		var customServer *httptest.Server

		BeforeEach(func() {
			fakeDbConn.QueryRowContextStub = dbHealthyStub
			dbComponentFactory.AllReturns(nil, nil)
			dbWorkerFactory.WorkersReturns([]db.Worker{
				makeDetailedWorker(db.WorkerStateRunning, "linux", "", nil),
				makeDetailedWorker(db.WorkerStateRunning, "linux", "", nil),
			}, nil)

			customServer = newApiTestServer(withMinWorkerThresholds(healthserver.MinWorkerThresholds{
				Platform: map[string]healthserver.ThresholdEntry{"linux": {Min: 2, Severity: healthserver.ThresholdSeverityCritical}},
			}))
		})

		AfterEach(func() {
			customServer.Close()
		})

		JustBeforeEach(func() {
			var err error
			response, err = client.Get(customServer.URL + "/api/v1/health")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 200 with status ok", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			body, _ := io.ReadAll(response.Body)
			var health atc.Health
			Expect(json.Unmarshal(body, &health)).To(Succeed())
			Expect(health.Status).To(Equal(atc.HealthStatusOK))
			Expect(health.Workers.Status).To(Equal(workerStatusHealthy))
		})
	})

	Context("with a per-platform threshold that is not met", func() {
		var customServer *httptest.Server

		BeforeEach(func() {
			fakeDbConn.QueryRowContextStub = dbHealthyStub
			dbComponentFactory.AllReturns(nil, nil)
			// Only 1 linux worker running, threshold requires 2
			dbWorkerFactory.WorkersReturns([]db.Worker{
				makeDetailedWorker(db.WorkerStateRunning, "linux", "", nil),
			}, nil)

			customServer = newApiTestServer(withMinWorkerThresholds(healthserver.MinWorkerThresholds{
				Platform: map[string]healthserver.ThresholdEntry{"linux": {Min: 2, Severity: healthserver.ThresholdSeverityCritical}},
			}))
		})

		AfterEach(func() {
			customServer.Close()
		})

		JustBeforeEach(func() {
			var err error
			response, err = client.Get(customServer.URL + "/api/v1/health")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 200 with status degraded", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			body, _ := io.ReadAll(response.Body)
			var health atc.Health
			Expect(json.Unmarshal(body, &health)).To(Succeed())
			Expect(health.Status).To(Equal(atc.HealthStatusDegraded))
			Expect(health.Workers.Status).To(Equal(workerStatusDegraded))
		})
	})

	Context("with a per-tag threshold targeting a tag absent from all workers", func() {
		var customServer *httptest.Server

		BeforeEach(func() {
			fakeDbConn.QueryRowContextStub = dbHealthyStub
			dbComponentFactory.AllReturns(nil, nil)
			// Workers running but none tagged "gpu"
			dbWorkerFactory.WorkersReturns([]db.Worker{
				makeDetailedWorker(db.WorkerStateRunning, "linux", "", nil),
			}, nil)

			customServer = newApiTestServer(withMinWorkerThresholds(healthserver.MinWorkerThresholds{
				Tag: map[string]healthserver.ThresholdEntry{"gpu": {Min: 1, Severity: healthserver.ThresholdSeverityCritical}},
			}))
		})

		AfterEach(func() {
			customServer.Close()
		})

		JustBeforeEach(func() {
			var err error
			response, err = client.Get(customServer.URL + "/api/v1/health")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 503 — absent tag with min>0 means 0 workers in that pool", func() {
			Expect(response.StatusCode).To(Equal(http.StatusServiceUnavailable))
			body, _ := io.ReadAll(response.Body)
			var health atc.Health
			Expect(json.Unmarshal(body, &health)).To(Succeed())
			Expect(health.Status).To(Equal(atc.HealthStatusFailing))
			Expect(health.Workers.Status).To(Equal(workerStatusUnhealthy))
			Expect(health.Workers.ByTag).To(HaveKey("gpu"))
			Expect(health.Workers.ByTag["gpu"].Running).To(Equal(0))
		})
	})

	Context("with a per-team threshold on unscoped workers (empty team name)", func() {
		var customServer *httptest.Server

		BeforeEach(func() {
			fakeDbConn.QueryRowContextStub = dbHealthyStub
			dbComponentFactory.AllReturns(nil, nil)
			// 2 unscoped workers running
			dbWorkerFactory.WorkersReturns([]db.Worker{
				makeDetailedWorker(db.WorkerStateRunning, "linux", "", nil),
				makeDetailedWorker(db.WorkerStateRunning, "linux", "", nil),
			}, nil)

			customServer = newApiTestServer(withMinWorkerThresholds(healthserver.MinWorkerThresholds{
				Team: map[string]healthserver.ThresholdEntry{"": {Min: 2, Severity: healthserver.ThresholdSeverityCritical}},
			}))
		})

		AfterEach(func() {
			customServer.Close()
		})

		JustBeforeEach(func() {
			var err error
			response, err = client.Get(customServer.URL + "/api/v1/health")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 200 with status ok when unscoped threshold is met", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			body, _ := io.ReadAll(response.Body)
			var health atc.Health
			Expect(json.Unmarshal(body, &health)).To(Succeed())
			Expect(health.Status).To(Equal(atc.HealthStatusOK))
			Expect(health.Workers.Status).To(Equal(workerStatusHealthy))
		})
	})

	Context("when a configured team pool has 0 running workers and min > 0", func() {
		var customServer *httptest.Server

		BeforeEach(func() {
			fakeDbConn.QueryRowContextStub = dbHealthyStub
			dbComponentFactory.AllReturns(nil, nil)
			dbWorkerFactory.WorkersReturns([]db.Worker{
				makeDetailedWorker(db.WorkerStateRunning, "linux", "", nil),
				makeDetailedWorker(db.WorkerStateStalled, "linux", "infra", nil),
			}, nil)

			customServer = newApiTestServer(withMinWorkerThresholds(healthserver.MinWorkerThresholds{
				Team: map[string]healthserver.ThresholdEntry{"infra": {Min: 1, Severity: healthserver.ThresholdSeverityCritical}},
			}))
		})

		AfterEach(func() {
			customServer.Close()
		})

		JustBeforeEach(func() {
			var err error
			response, err = client.Get(customServer.URL + "/api/v1/health")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 503 — 0 running workers in a configured team pool", func() {
			Expect(response.StatusCode).To(Equal(http.StatusServiceUnavailable))
			body, _ := io.ReadAll(response.Body)
			var health atc.Health
			Expect(json.Unmarshal(body, &health)).To(Succeed())
			Expect(health.Status).To(Equal(atc.HealthStatusFailing))
			Expect(health.Workers.Status).To(Equal(workerStatusUnhealthy))
			Expect(health.Workers.ByTeam).To(HaveKey("infra"))
			Expect(health.Workers.ByTeam["infra"].Running).To(Equal(0))
		})
	})

	Context("when a configured class threshold has min=0 and the class has no running workers", func() {
		var customServer *httptest.Server

		BeforeEach(func() {
			fakeDbConn.QueryRowContextStub = dbHealthyStub
			dbComponentFactory.AllReturns(nil, nil)
			dbWorkerFactory.WorkersReturns([]db.Worker{
				makeDetailedWorker(db.WorkerStateRunning, "linux", "", nil),
			}, nil)

			customServer = newApiTestServer(withMinWorkerThresholds(healthserver.MinWorkerThresholds{
				Tag: map[string]healthserver.ThresholdEntry{"gpu": {Min: 0, Severity: healthserver.ThresholdSeverityCritical}},
			}))
		})

		AfterEach(func() {
			customServer.Close()
		})

		JustBeforeEach(func() {
			var err error
			response, err = client.Get(customServer.URL + "/api/v1/health")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 200 ok — min=0 disables the check for that class", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			body, _ := io.ReadAll(response.Body)
			var health atc.Health
			Expect(json.Unmarshal(body, &health)).To(Succeed())
			Expect(health.Status).To(Equal(atc.HealthStatusOK))
			Expect(health.Workers.Status).To(Equal(workerStatusHealthy))
		})
	})

	Context("with a warn-severity threshold and zero workers in that pool", func() {
		var customServer *httptest.Server

		BeforeEach(func() {
			fakeDbConn.QueryRowContextStub = dbHealthyStub
			dbComponentFactory.AllReturns(nil, nil)
			dbWorkerFactory.WorkersReturns([]db.Worker{
				makeDetailedWorker(db.WorkerStateRunning, "linux", "", nil),
			}, nil)

			customServer = newApiTestServer(withMinWorkerThresholds(healthserver.MinWorkerThresholds{
				Tag: map[string]healthserver.ThresholdEntry{"gpu": {Min: 1, Severity: healthserver.ThresholdSeverityWarn}},
			}))
		})

		AfterEach(func() {
			customServer.Close()
		})

		JustBeforeEach(func() {
			var err error
			response, err = client.Get(customServer.URL + "/api/v1/health")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 200 degraded — warn never triggers failing even at zero", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			body, _ := io.ReadAll(response.Body)
			var health atc.Health
			Expect(json.Unmarshal(body, &health)).To(Succeed())
			Expect(health.Status).To(Equal(atc.HealthStatusDegraded))
			Expect(health.Workers.Status).To(Equal(workerStatusDegraded))
		})
	})

	Context("with an info-severity threshold and zero workers in that pool", func() {
		var customServer *httptest.Server

		BeforeEach(func() {
			fakeDbConn.QueryRowContextStub = dbHealthyStub
			dbComponentFactory.AllReturns(nil, nil)
			dbWorkerFactory.WorkersReturns([]db.Worker{
				makeDetailedWorker(db.WorkerStateRunning, "linux", "", nil),
			}, nil)

			customServer = newApiTestServer(withMinWorkerThresholds(healthserver.MinWorkerThresholds{
				Tag: map[string]healthserver.ThresholdEntry{"gpu": {Min: 1, Severity: healthserver.ThresholdSeverityInfo}},
			}))
		})

		AfterEach(func() {
			customServer.Close()
		})

		JustBeforeEach(func() {
			var err error
			response, err = client.Get(customServer.URL + "/api/v1/health")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 200 ok — info never affects status", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			body, _ := io.ReadAll(response.Body)
			var health atc.Health
			Expect(json.Unmarshal(body, &health)).To(Succeed())
			Expect(health.Status).To(Equal(atc.HealthStatusOK))
			Expect(health.Workers.Status).To(Equal(workerStatusHealthy))
			Expect(health.Workers.ByTag).To(HaveKey("gpu"))
			Expect(health.Workers.ByTag["gpu"].Running).To(Equal(0))
		})
	})
})
