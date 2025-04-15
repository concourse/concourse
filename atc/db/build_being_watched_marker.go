package db

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagerctx"
	sq "github.com/Masterminds/squirrel"
)

// beingWatchedBuildEventChannelMap stores build event notifier channel names
// for those builds that are being watched. The way to know if a build is being
// watched is that, when a build is watched on UI, then build.Events() will be
// called. So that we can mark a build as BeingWatched from build.Events(). Note
// that, as build event notification should only be sent from running builds,
// this map should only store running builds' event channel names.
type beingWatchedBuildEventChannelMap struct {
	internal sync.Map // string -> time.Time
}

var (
	beingWatchedBuildEventMap *beingWatchedBuildEventChannelMap
	once                      sync.Once
)

// NewBeingWatchedBuildEventChannelMap returns a singleton instance of
// beingWatchedBuildEventChannelMap.
func NewBeingWatchedBuildEventChannelMap() *beingWatchedBuildEventChannelMap {
	once.Do(func() {
		beingWatchedBuildEventMap = &beingWatchedBuildEventChannelMap{}
	})
	return beingWatchedBuildEventMap
}

func (m *beingWatchedBuildEventChannelMap) Mark(buildEventChannel string, t time.Time) {
	m.internal.Store(buildEventChannel, t)
}

// BeingWatched returns true if given buildEventChannel is being watched.
func (m *beingWatchedBuildEventChannelMap) BeingWatched(buildEventChannel string) bool {
	_, ok := m.internal.Load(buildEventChannel)
	return ok
}

// Clean deletes entries based on conditionFunc returning true.
func (m *beingWatchedBuildEventChannelMap) Clean(conditionFunc func(k string, v time.Time) bool) {
	m.internal.Range(func(key, value any) bool {
		k := key.(string)
		v := value.(time.Time)
		if conditionFunc(k, v) {
			m.internal.Delete(k)
		}
		return true
	})
}

const beingWatchedNotifyChannelName = "being_watched_build_event_channel"

// MarkBuildAsBeingWatched marks a build as BeingWatched by sending a db
// notification to channel beingWatchedNotifyChannelName with payload of
// the build's event channel name. This is because a build may be watched
// from any ATCs, while the build may be running in a separate ATC.
func MarkBuildAsBeingWatched(db DbConn, buildEventChannel string) error {
	_, err := db.Exec(fmt.Sprintf("NOTIFY %s, '%s'", beingWatchedNotifyChannelName, buildEventChannel))
	if err != nil {
		return err
	}
	return nil
}

// BuildBeingWatchedMarker listens to channel beingWatchedNotifyChannelName and
// mark builds as BeingWatched accordingly in a singleton map. And it periodically
// cleans up the map.
type BuildBeingWatchedMarker struct {
	conn               DbConn
	dataRetainDuration time.Duration
	watchedMap         *beingWatchedBuildEventChannelMap
	notifier           chan Notification
	clock              clock.Clock
	wg                 *sync.WaitGroup
	stop               chan struct{}
}

const DefaultBuildBeingWatchedMarkDuration = 2 * time.Hour

func NewBuildBeingWatchedMarker(logger lager.Logger, conn DbConn, dataRetainDuration time.Duration, clock clock.Clock) (*BuildBeingWatchedMarker, error) {
	if dataRetainDuration < 0 {
		return nil, errors.New("data retain duration must be positive")
	}

	w := &BuildBeingWatchedMarker{
		conn:               conn,
		dataRetainDuration: dataRetainDuration,
		watchedMap:         NewBeingWatchedBuildEventChannelMap(),
		clock:              clock,
		wg:                 new(sync.WaitGroup),
		stop:               make(chan struct{}, 1),
	}

	notifier, err := w.conn.Bus().Listen(beingWatchedNotifyChannelName, 100)
	if err != nil {
		return nil, err
	}
	w.notifier = notifier

	w.wg.Add(1)
	go func(logger lager.Logger, w *BuildBeingWatchedMarker) {
		defer w.wg.Done()
		defer w.conn.Bus().Unlisten(beingWatchedNotifyChannelName, notifier)

		for {
			select {
			case notification := <-w.notifier:
				w.watchedMap.Mark(notification.Payload, w.clock.Now())
				logger.Debug("start-to-watch-build", lager.Data{"channel": notification.Payload})
			case <-w.stop:
				return
			}
		}
	}(logger, w)

	return w, nil
}

// Run is periodically invoked to clean the internal map. We have no way to
// know if a build is no longer watched by any client, so cleanup strategy
// is, after a build is added to the map, we keep it in the map for 2 hours.
// After 2 hours, we will query its status. If it's completed, then we delete
// it from the map. If we cannot find the build, mostly like that's a check
// build, as a check build should never last 2 hours, so we just delete it
// from the map.
func (bt *BuildBeingWatchedMarker) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx)

	logger.Debug("start")
	defer logger.Debug("done")

	bt.watchedMap.Clean(func(k string, v time.Time) bool {
		if v.After(bt.clock.Now().Add(-bt.dataRetainDuration)) {
			return false
		}
		return bt.isBuildCompleted(k)
	})

	return nil
}

func (bt *BuildBeingWatchedMarker) Drain(ctx context.Context) {
	logger := lagerctx.FromContext(ctx)

	logger.Debug("start")
	defer logger.Debug("done")

	close(bt.stop)
	bt.wg.Wait()
}

func (bt *BuildBeingWatchedMarker) isBuildCompleted(channel string) bool {
	strBuildID := strings.TrimPrefix(channel, buildEventChannelPrefix)
	buildID, err := strconv.Atoi(strBuildID)
	if err != nil {
		// If build id is not an integer, then we consider a wrong channel,
		// so return true to delete it.
		return true
	}

	completed := false
	err = psql.Select("completed").
		From("builds").
		Where(sq.Eq{"id": buildID}).
		RunWith(bt.conn).
		QueryRow().
		Scan(&completed)
	if err != nil {
		// If we cannot get a build's status, then we consider the build is
		// no longer being watched.
		return true
	}
	return completed
}
