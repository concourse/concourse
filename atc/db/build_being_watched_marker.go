package db

import (
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagerctx"
	"context"
	"fmt"
	"sync"
	"time"
)

// beingWatchedBuildEventChannelMap stores build event notifier channel names
// for those builds that are being watched. The way to know if a build is being
// watched is that, when a build is watched on UI, then build.Events() will be
// called. So that we can mark a build as BeingWatched from build.Events(). Note
// that, as build event notification should only be sent from running builds,
// this map should only store running builds' event channel names.
type beingWatchedBuildEventChannelMap struct {
	sync.RWMutex
	internal map[string]time.Time
}

var (
	beingWatchedBuildEventMap *beingWatchedBuildEventChannelMap
	once                      sync.Once
)

// NewBeingWatchedBuildEventChannelMap returns a singleton instance of
// beingWatchedBuildEventChannelMap.
func NewBeingWatchedBuildEventChannelMap() *beingWatchedBuildEventChannelMap {
	once.Do(func() {
		beingWatchedBuildEventMap = &beingWatchedBuildEventChannelMap{
			internal: make(map[string]time.Time),
		}
	})
	return beingWatchedBuildEventMap
}

func (m *beingWatchedBuildEventChannelMap) load(key string) (value time.Time, ok bool) {
	m.RLock()
	result, ok := m.internal[key]
	m.RUnlock()
	return result, ok
}

func (m *beingWatchedBuildEventChannelMap) delete(key string) {
	m.Lock()
	delete(m.internal, key)
	m.Unlock()
}

func (m *beingWatchedBuildEventChannelMap) store(key string, value time.Time) {
	m.Lock()
	m.internal[key] = value
	m.Unlock()
}

// BeingWatched returns true if given buildEventChannel is being watched.
func (m *beingWatchedBuildEventChannelMap) BeingWatched(buildEventChannel string) bool {
	_, ok := beingWatchedBuildEventMap.load(buildEventChannel)
	return ok
}

func (m *beingWatchedBuildEventChannelMap) Clean(conditionFunc func(k string, v time.Time) (string, bool)) {
	var toClean []string
	m.RLock()
	for k, v := range m.internal {
		kk, do := conditionFunc(k, v)
		if do {
			toClean = append(toClean, kk)
		}
	}
	m.RUnlock()

	for _, k := range toClean {
		m.delete(k)
	}
}

const beingWatchedNotifyChannelName = "being_watched_build_event_channel"

// markBuildAsBeingWatched marks a build as BeingWatched by sending a db
// notification to channel beingWatchedNotifyChannelName with payload of
// the build's event channel name. This is because a build may be watched
// from any ATCs, while the build may be running in a separate ATC.
func markBuildAsBeingWatched(db Conn, buildEventChannel string) error {
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
	bus        NotificationsBus
	watchedMap *beingWatchedBuildEventChannelMap
	notifier   chan Notification
}

func NewBuildEventWatcher(logger lager.Logger, bus NotificationsBus) (*BuildBeingWatchedMarker, error) {
	w := &BuildBeingWatchedMarker{
		bus:        bus,
		watchedMap: NewBeingWatchedBuildEventChannelMap(),
	}

	notifier, err := w.bus.Listen(beingWatchedNotifyChannelName, 100)
	if err != nil {
		return nil, err
	}
	w.notifier = notifier

	go func(logger lager.Logger, notifier chan Notification) {
		defer w.bus.Unlisten(beingWatchedNotifyChannelName, notifier)

		for {
			notification, ok := <-notifier
			if !ok {
				return
			}

			beingWatchedBuildEventMap.store(notification.Payload, time.Now())
			logger.Debug("EVAN: start-to-watch-build", lager.Data{"channel": notification.Payload})
		}
	}(logger, w.notifier)

	return w, nil
}

func (bt *BuildBeingWatchedMarker) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx)

	logger.Debug("start")
	defer logger.Debug("done")

	bt.watchedMap.Clean(func(k string, v time.Time) (string, bool) {
		if v.Before(time.Now().Add(-2 * time.Hour)) {
			return k, true
		}
		return k, false
	})

	return nil
}

func (bt *BuildBeingWatchedMarker) Drain(ctx context.Context) {
	logger := lagerctx.FromContext(ctx)
	logger.Info("close-being-watched-build-marker")
	close(bt.notifier)
}
