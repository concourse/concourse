package watch

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"code.cloudfoundry.org/lager"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
)

type ListAllJobsEvent struct {
	ID   int
	Type EventType
	Job  *atc.DashboardJob
}

type ListAllJobsWatcher struct {
	logger      lager.Logger
	conn        db.Conn
	lockFactory lock.LockFactory

	mtx         sync.RWMutex
	subscribers map[chan []ListAllJobsEvent]struct{}
}

var listAllJobsWatchTables = []watchTable{
	{
		table: "jobs",
		idCol: "id",

		insert: true,
		update: true,
		updateCols: []string{
			"name", "active", "paused", "has_new_inputs", "tags", "latest_completed_build_id",
			"next_build_id", "transition_build_id", "config",
		},
		delete: true,
	},
	{
		table: "pipelines",
		idCol: "id",

		update:     true,
		updateCols: []string{"name", "public"},
	},
	{
		table: "teams",
		idCol: "id",

		update:     true,
		updateCols: []string{"name"},
	},
}

func NewListAllJobsWatcher(logger lager.Logger, conn db.Conn, lockFactory lock.LockFactory) (*ListAllJobsWatcher, error) {
	watcher := &ListAllJobsWatcher{
		logger:      logger,
		conn:        conn,
		lockFactory: lockFactory,

		subscribers: make(map[chan []ListAllJobsEvent]struct{}),
	}

	if err := watcher.setupTriggers(); err != nil {
		return nil, fmt.Errorf("setup triggers: %w", err)
	}

	notifs, err := watcher.conn.Bus().Listen(eventsChannel, db.QueueNotifications)
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	go watcher.drain(notifs)

	return watcher, nil
}

func (w *ListAllJobsWatcher) setupTriggers() error {
	l, acquired, err := w.lockFactory.Acquire(w.logger, lock.NewCreateWatchTriggersLockID())
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	if !acquired {
		w.logger.Debug("lock-already-held")
		return nil
	}
	defer l.Release()

	tx, err := w.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	if _, err = tx.Exec(createNotifyTriggerFunction); err != nil {
		return fmt.Errorf("create notify function: %w", err)
	}

	for _, tbl := range listAllJobsWatchTables {
		if err = createWatchEventsTrigger(tx, tbl); err != nil {
			return fmt.Errorf("create trigger for %s: %w", tbl.table, err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (w *ListAllJobsWatcher) WatchListAllJobs(ctx context.Context, access accessor.Access) <-chan []ListAllJobsEvent {
	eventsChan := make(chan []ListAllJobsEvent)

	dirty := make(chan struct{})
	var pendingEvents []ListAllJobsEvent
	var mtx sync.Mutex
	go w.watchEvents(ctx, access, &pendingEvents, &mtx, dirty)
	go w.sendEvents(ctx, eventsChan, &pendingEvents, &mtx, dirty)
	return eventsChan
}

func (w *ListAllJobsWatcher) watchEvents(
	ctx context.Context,
	access accessor.Access,
	pendingEvents *[]ListAllJobsEvent,
	mtx *sync.Mutex,
	dirty chan<- struct{},
) {
	c := w.subscribe()
	defer w.unsubscribe(c)
	for {
		select {
		case <-ctx.Done():
			return
		case evts, ok := <-c:
			if !ok {
				return
			}
			mtx.Lock()
			for _, evt := range evts {
				if w.hasAccessTo(access, evt) {
					*pendingEvents = append(*pendingEvents, evt)
				}
			}
			if len(*pendingEvents) > 0 {
				invalidate(dirty)
			}
			mtx.Unlock()
		}
	}
}

func (w *ListAllJobsWatcher) sendEvents(
	ctx context.Context,
	eventsChan chan<- []ListAllJobsEvent,
	pendingEvents *[]ListAllJobsEvent,
	mtx *sync.Mutex,
	dirty <-chan struct{},
) {
	defer close(eventsChan)
	for {
		select {
			case <-ctx.Done():
				return
			case <-dirty:
		}
		mtx.Lock()
		eventsToSend := make([]ListAllJobsEvent, len(*pendingEvents))
		copy(eventsToSend, *pendingEvents)
		*pendingEvents = (*pendingEvents)[:0]
		mtx.Unlock()

		select {
		case eventsChan <- eventsToSend:
		case <-ctx.Done():
			return
		}
	}
}

func invalidate(dirty chan<- struct{}) {
	select {
	case dirty <- struct{}{}:
	default:
	}
}

func (w *ListAllJobsWatcher) subscribe() chan []ListAllJobsEvent {
	c := make(chan []ListAllJobsEvent)

	w.mtx.Lock()
	defer w.mtx.Unlock()
	w.subscribers[c] = struct{}{}

	return c
}

func (w *ListAllJobsWatcher) unsubscribe(c chan []ListAllJobsEvent) {
	w.mtx.Lock()
	defer w.mtx.Unlock()
	delete(w.subscribers, c)
}

func (w *ListAllJobsWatcher) noSubscribers() bool {
	w.mtx.RLock()
	defer w.mtx.RUnlock()
	return len(w.subscribers) == 0
}

func (w *ListAllJobsWatcher) terminateSubscribers() {
	w.mtx.Lock()
	defer w.mtx.Unlock()
	for c := range w.subscribers {
		close(c)
		delete(w.subscribers, c)
	}
}

func (w *ListAllJobsWatcher) hasAccessTo(access accessor.Access, evt ListAllJobsEvent) bool {
	if access.IsAdmin() {
		return true
	}
	if evt.Job == nil {
		// this means we send DELETE events to all subscribers.
		// given that there's no sensitive information (just the id, which is serial anyway), I suspect this is okay
		return true
	}
	// TODO: what about jobs from public pipelines?
	for _, teamName := range access.TeamNames() {
		if evt.Job.TeamName == teamName {
			return true
		}
	}
	return false
}

func (w *ListAllJobsWatcher) drain(notifs chan db.Notification) {
	for notif := range notifs {
		if notif.Healthy {
			if err := w.process(notif.Payload); err != nil {
				w.logger.Error("process-notification", err, lager.Data{"payload": notif.Payload})
			}
		} else {
			w.terminateSubscribers()
		}
	}
}

func (w *ListAllJobsWatcher) process(payload string) error {
	if w.noSubscribers() {
		return nil
	}
	var notif Notification
	err := json.Unmarshal([]byte(payload), &notif)
	if err != nil {
		return err
	}
	var pred interface{}
	var jobID int
	switch notif.Table {
	case "jobs":
		jobID, pred, err = intEqPred("j.id", notif.Data["id"])
		if notif.Operation == "DELETE" {
			w.publishEvents(ListAllJobsEvent{
				ID:   jobID,
				Type: Delete,
			})
			return nil
		}
	case "pipelines":
		_, pred, err = intEqPred("p.id", notif.Data["id"])
	case "teams":
		_, pred, err = intEqPred("tm.id", notif.Data["id"])
	default:
		return nil
	}
	if err != nil {
		return err
	}
	jobs, err := w.fetchJobs(pred)
	if err != nil {
		return err
	}
	if len(jobs) == 0 {
		// an update to a job that results in it not being found is updating active to false (or it was already false).
		// either way, sending a 'DELETE' is reasonable, as long as we make no guarantees about repeated DELETEs
		if notif.Table == "jobs" && notif.Operation == "UPDATE" {
			w.publishEvents(ListAllJobsEvent{
				ID:   jobID,
				Type: Delete,
			})
		}
		return nil
	}
	evts := make([]ListAllJobsEvent, len(jobs))
	for i, job := range jobs {
		evts[i] = ListAllJobsEvent{
			ID:   job.ID,
			Type: Put,
			Job:  &jobs[i],
		}
	}
	w.publishEvents(evts...)
	return nil
}

func (w *ListAllJobsWatcher) fetchJobs(pred interface{}) (atc.Dashboard, error) {
	tx, err := w.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	factory := db.NewDashboardFactory(tx, pred)
	dashboard, err := factory.BuildDashboard()
	if err != nil {
		return nil, err
	}
	err = tx.Commit()
	if err != nil {
		return nil, err
	}
	return dashboard, nil
}

func (w *ListAllJobsWatcher) publishEvents(evts ...ListAllJobsEvent) {
	w.mtx.RLock()
	defer w.mtx.RUnlock()
	for c := range w.subscribers {
		c <- evts
	}
}

func intEqPred(col string, raw string) (int, interface{}, error) {
	val, err := strconv.Atoi(raw)
	if err != nil {
		return 0, nil, err
	}
	return val, sq.Eq{col: val}, nil
}
