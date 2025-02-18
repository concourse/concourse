package db

import (
	"encoding/json"
	"sync"
	"time"

	"code.cloudfoundry.org/lager/v3"
	sq "github.com/Masterminds/squirrel"
)

// WorkerCache monitors changes to the workers and containers tables. It keeps
// a list of workers and the number of active build containers that belong to
// each worker in memory. The container count is from the perspective of the
// DB, rather than what the workers report.
//
// In addition to responding to state changes, we also periodically re-sync the
// data by fetching fresh data from the database. Theoretically this shouldn't
// be necessary (since we try to respond to every change to the tables), but
// it's possible to miss events (e.g. the notification bus queue is full,
// network flakes, etc).
type WorkerCache struct {
	conn   DbConn
	logger lager.Logger

	// mut is used to synchronize access to the cached data and lastRefresh
	mut sync.RWMutex

	lastRefresh     time.Time
	refreshInterval time.Duration

	// Cached data
	workers               map[string]Worker
	workerContainerCounts map[string]int
}

type TriggerEvent struct {
	Operation string             `json:"operation"`
	Data      map[string]*string `json:"data"`
}

func NewWorkerCache(logger lager.Logger, conn DbConn, refreshInterval time.Duration) (*WorkerCache, error) {
	cache := NewStaticWorkerCache(logger, conn, refreshInterval)

	workerNotifs, err := conn.Bus().Listen("worker_events_channel", 128)
	if err != nil {
		return nil, err
	}

	containerNotifs, err := conn.Bus().Listen("container_events_channel", 512)
	if err != nil {
		return nil, err
	}

	go cache.listenWorkers(workerNotifs)
	go cache.listenContainers(containerNotifs)

	return cache, nil
}

// NewStaticWorkerCache returns a WorkerCache that doesn't subscribe to changes
// to the workers/containers table, so it's data is likely to be stale until
// the next refresh.
func NewStaticWorkerCache(logger lager.Logger, conn DbConn, refreshInterval time.Duration) *WorkerCache {
	return &WorkerCache{
		logger:                logger,
		conn:                  conn,
		refreshInterval:       refreshInterval,
		workers:               make(map[string]Worker),
		workerContainerCounts: make(map[string]int),
	}
}

func (cache *WorkerCache) Workers() ([]Worker, error) {
	// need to hold the RLock for accessing lastRefresh
	cache.mut.RLock()
	if time.Since(cache.lastRefresh) >= cache.refreshInterval {
		cache.mut.RUnlock()
		if err := cache.refreshWorkerData(); err != nil {
			return nil, err
		}
		cache.mut.RLock()
	}
	defer cache.mut.RUnlock()

	workers := make([]Worker, 0, len(cache.workers))
	for _, worker := range cache.workers {
		workers = append(workers, worker)
	}

	return workers, nil
}

func (cache *WorkerCache) WorkerContainerCounts() (map[string]int, error) {
	// need to hold the RLock for accessing lastRefresh
	cache.mut.RLock()
	if cache.needsRefresh() {
		cache.mut.RUnlock()
		if err := cache.refreshWorkerData(); err != nil {
			return nil, err
		}
		cache.mut.RLock()
	}
	defer cache.mut.RUnlock()

	workerContainerCounts := make(map[string]int, len(cache.workerContainerCounts))
	for workerName, count := range cache.workerContainerCounts {
		workerContainerCounts[workerName] = count
	}

	return workerContainerCounts, nil
}

func (cache *WorkerCache) listenWorkers(notifications <-chan Notification) {
	logger := cache.logger.Session("listen-workers")

	for notification := range notifications {
		if !notification.Healthy {
			logger.Info("notification-unhealthy-will-refresh")
			cache.ensureRefresh()
			continue
		}

		var event TriggerEvent
		if err := json.Unmarshal([]byte(notification.Payload), &event); err != nil {
			logger.Error("invalid-payload", err)
			continue
		}

		workerName := event.Data["name"]
		if workerName == nil {
			logger.Info("missing-name")
			continue
		}

		if event.Operation == "UPDATE" || event.Operation == "INSERT" {
			if err := cache.upsertWorker(*workerName); err != nil {
				logger.Error("failed-to-upsert-worker", err)
				continue
			}
		} else if event.Operation == "DELETE" {
			cache.removeWorker(*workerName)
		} else {
			logger.Info("unexpected-operation", lager.Data{"operation": event.Operation})
		}
	}
}

func (cache *WorkerCache) listenContainers(notifications <-chan Notification) {
	logger := cache.logger.Session("listen-containers")

	for notification := range notifications {
		if !notification.Healthy {
			logger.Info("notification-unhealthy-will-refresh")
			cache.ensureRefresh()
			continue
		}

		var event TriggerEvent
		if err := json.Unmarshal([]byte(notification.Payload), &event); err != nil {
			logger.Error("invalid-payload", err)
			continue
		}

		workerName := event.Data["worker_name"]
		if workerName == nil {
			logger.Info("missing-name")
			continue
		}

		if event.Data["build_id"] == nil {
			// Skip over non-build containers.
			continue
		}

		if event.Operation == "INSERT" {
			cache.addWorkerContainerCount(*workerName, 1)
		} else if event.Operation == "DELETE" {
			cache.addWorkerContainerCount(*workerName, -1)
		} else {
			logger.Info("unexpected-operation", lager.Data{"operation": event.Operation})
		}
	}
}

func (cache *WorkerCache) refreshWorkerData() error {
	cache.mut.Lock()
	defer cache.mut.Unlock()

	if !cache.needsRefresh() {
		return nil
	}

	cache.logger.Debug("refreshing")

	workers, err := getWorkers(cache.conn, workersQuery)
	if err != nil {
		return err
	}

	cache.workers = make(map[string]Worker, len(workers))
	for _, worker := range workers {
		cache.workers[worker.Name()] = worker
	}

	rows, err := psql.Select("worker_name, COUNT(*)").
		From("containers").
		Where("build_id IS NOT NULL").
		GroupBy("worker_name").
		RunWith(cache.conn).
		Query()
	if err != nil {
		return err
	}

	defer Close(rows)

	countByWorker := make(map[string]int, len(workers))

	for rows.Next() {
		var workerName string
		var containersCount int

		err = rows.Scan(&workerName, &containersCount)
		if err != nil {
			return err
		}

		countByWorker[workerName] = containersCount
	}

	cache.lastRefresh = time.Now()
	cache.workerContainerCounts = countByWorker

	return nil
}

func (cache *WorkerCache) needsRefresh() bool {
	return time.Since(cache.lastRefresh) >= cache.refreshInterval
}

func (cache *WorkerCache) ensureRefresh() {
	cache.mut.Lock()
	defer cache.mut.Unlock()

	cache.lastRefresh = time.Time{}
}

func (cache *WorkerCache) removeWorker(name string) {
	cache.mut.Lock()
	defer cache.mut.Unlock()

	delete(cache.workers, name)
	delete(cache.workerContainerCounts, name)
}

func (cache *WorkerCache) upsertWorker(name string) error {
	worker, found, err := getWorker(cache.conn, workersQuery.Where(sq.Eq{"w.name": name}))
	if err != nil {
		return err
	}
	if !found {
		// worker disappeared while trying to fetch, so ensure it's gone
		cache.removeWorker(name)
		return nil
	}

	cache.mut.Lock()
	defer cache.mut.Unlock()

	cache.workers[name] = worker
	if _, ok := cache.workerContainerCounts[name]; !ok {
		cache.workerContainerCounts[name] = 0
	}
	return nil
}

func (cache *WorkerCache) addWorkerContainerCount(workerName string, delta int) {
	cache.mut.Lock()
	defer cache.mut.Unlock()

	cache.workerContainerCounts[workerName] += delta
}
