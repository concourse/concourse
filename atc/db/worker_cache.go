package db

import (
	"encoding/json"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/lager/v3"
	sq "github.com/Masterminds/squirrel"
)

// WorkerCache provides a thread-safe, in-memory cache of worker information
// from the database. It tracks both worker metadata and container counts,
// refreshing periodically and in response to notification events.
//
// The implementation handles three key challenges:
// 1. Race conditions between database events and cached state
// 2. Concurrent access from multiple goroutines
// 3. Efficient updates without excessive locking
//
// Cache consistency is maintained by:
// - Responding to worker and container table events via notification channels
// - Periodically refreshing the entire dataset to recover from missed events
// - Using appropriate locking to prevent concurrent state corruption
//
// The refreshing strategy uses atomic operations and separate locks to
// minimize contention while ensuring safe, consistent state updates.
type WorkerCache struct {
	conn   DbConn
	logger lager.Logger

	// dataMut protects access to the cached data
	dataMut sync.RWMutex

	// refreshMut protects the refresh operation itself
	refreshMut sync.Mutex

	// Use atomic for refresh state to avoid locking
	refreshing      atomic.Bool
	lastRefresh     atomic.Int64 // Unix timestamp in nanoseconds
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
	cache := &WorkerCache{
		logger:                logger,
		conn:                  conn,
		refreshInterval:       refreshInterval,
		workers:               make(map[string]Worker),
		workerContainerCounts: make(map[string]int),
	}

	// Set initial refresh time to zero
	cache.lastRefresh.Store(0)

	return cache
}

// Copy maps safely for returning to caller
func (cache *WorkerCache) copyWorkers() []Worker {
	cache.dataMut.RLock()
	defer cache.dataMut.RUnlock()

	workers := make([]Worker, 0, len(cache.workers))
	for _, worker := range cache.workers {
		workers = append(workers, worker)
	}
	return workers
}

func (cache *WorkerCache) copyWorkerContainerCounts() map[string]int {
	cache.dataMut.RLock()
	defer cache.dataMut.RUnlock()

	counts := make(map[string]int, len(cache.workerContainerCounts))
	maps.Copy(counts, cache.workerContainerCounts)
	return counts
}

func (cache *WorkerCache) Workers() ([]Worker, error) {
	if cache.needsRefresh() {
		err := cache.refreshWorkerData()
		if err != nil {
			return nil, err
		}
	}

	return cache.copyWorkers(), nil
}

func (cache *WorkerCache) WorkerContainerCounts() (map[string]int, error) {
	if cache.needsRefresh() {
		err := cache.refreshWorkerData()
		if err != nil {
			return nil, err
		}
	}

	return cache.copyWorkerContainerCounts(), nil
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
	// Use a mutex to ensure only one refresh happens at a time
	cache.refreshMut.Lock()
	defer cache.refreshMut.Unlock()

	// Double-check if refresh is still needed after acquiring lock
	if !cache.needsRefresh() {
		return nil
	}

	// Set refreshing flag
	if !cache.startRefresh() {
		// Another goroutine is already refreshing
		return nil
	}

	defer cache.endRefresh()

	cache.logger.Debug("refreshing")

	// Perform DB queries outside of the data mutex
	workers, err := getWorkers(cache.conn, workersQuery)
	if err != nil {
		return err
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

	// Prepare new maps
	newWorkers := make(map[string]Worker, len(workers))
	for _, worker := range workers {
		newWorkers[worker.Name()] = worker
	}

	newCountByWorker := make(map[string]int, len(workers))
	for rows.Next() {
		var workerName string
		var containersCount int

		err = rows.Scan(&workerName, &containersCount)
		if err != nil {
			return err
		}

		newCountByWorker[workerName] = containersCount
	}

	cache.dataMut.Lock()
	cache.workers = newWorkers
	cache.workerContainerCounts = newCountByWorker
	cache.dataMut.Unlock()

	cache.lastRefresh.Store(time.Now().UnixNano())

	return nil
}

func (cache *WorkerCache) needsRefresh() bool {
	lastRefreshTime := time.Unix(0, cache.lastRefresh.Load())
	return time.Since(lastRefreshTime) >= cache.refreshInterval
}

func (cache *WorkerCache) startRefresh() bool {
	return cache.refreshing.CompareAndSwap(false, true)
}

func (cache *WorkerCache) endRefresh() {
	cache.refreshing.Store(false)
}

func (cache *WorkerCache) ensureRefresh() {
	cache.lastRefresh.Store(0)
}

func (cache *WorkerCache) removeWorker(name string) {
	cache.dataMut.Lock()
	defer cache.dataMut.Unlock()

	delete(cache.workers, name)
	delete(cache.workerContainerCounts, name)
}

func (cache *WorkerCache) upsertWorker(name string) error {
	worker, found, err := getWorker(cache.conn, workersQuery.Where(sq.Eq{"w.name": name}))
	if err != nil {
		return err
	}

	if !found {
		cache.removeWorker(name)
		return nil
	}

	cache.dataMut.Lock()
	defer cache.dataMut.Unlock()

	cache.workers[name] = worker
	// Only initialize container count if not present
	if _, ok := cache.workerContainerCounts[name]; !ok {
		cache.workerContainerCounts[name] = 0
	}
	return nil
}

func (cache *WorkerCache) addWorkerContainerCount(workerName string, delta int) {
	cache.dataMut.Lock()
	defer cache.dataMut.Unlock()

	// Only update count if worker exists in cache
	if _, exists := cache.workers[workerName]; exists {
		cache.workerContainerCounts[workerName] += delta
	}
}
