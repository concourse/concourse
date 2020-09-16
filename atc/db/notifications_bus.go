package db

import (
	"database/sql"
	"sync"

	"github.com/lib/pq"
)

//go:generate counterfeiter . Listener

type Listener interface {
	Close() error
	Listen(channel string) error
	Unlisten(channel string) error
	NotificationChannel() <-chan *pq.Notification
}

//go:generate counterfeiter . Executor

type Executor interface {
	Exec(statement string, args ...interface{}) (sql.Result, error)
}

type NotificationsBus interface {
	Notify(channel string) error
	Listen(channel string) (chan bool, error)
	Unlisten(channel string, notify chan bool) error
	Close() error
}

type notificationsBus struct {
	sync.Mutex

	listener Listener
	executor Executor

	notifications *notificationsMap
}

func NewNotificationsBus(listener Listener, executor Executor) *notificationsBus {
	bus := &notificationsBus{
		listener:      listener,
		executor:      executor,
		notifications: newNotificationsMap(),
	}

	go bus.wait()

	return bus
}

func (bus *notificationsBus) Close() error {
	return bus.listener.Close()
}

func (bus *notificationsBus) Notify(channel string) error {
	_, err := bus.executor.Exec("NOTIFY " + channel)
	return err
}

func (bus *notificationsBus) Listen(channel string) (chan bool, error) {
	bus.Lock()
	defer bus.Unlock()

	if bus.notifications.empty(channel) {
		err := bus.listener.Listen(channel)
		if err != nil {
			return nil, err
		}
	}

	notify := make(chan bool, 1)
	bus.notifications.register(channel, notify)
	return notify, nil
}

func (bus *notificationsBus) Unlisten(channel string, notify chan bool) error {
	bus.Lock()
	defer bus.Unlock()

	bus.notifications.unregister(channel, notify)

	if bus.notifications.empty(channel) {
		return bus.listener.Unlisten(channel)
	}

	return nil
}

func (bus *notificationsBus) wait() {
	for {
		notification, ok := <-bus.listener.NotificationChannel()
		if !ok {
			break
		}

		if notification != nil {
			bus.handleNotification(notification)
		} else {
			bus.handleReconnect()
		}
	}
}

func (bus *notificationsBus) handleNotification(notification *pq.Notification) {
	// alert any relevant listeners of notification being received
	// (nonblocking)
	bus.notifications.eachForChannel(notification.Channel, func(sink chan bool) {
		select {
		case sink <- true:
			// notified of message being received (or queued up)
		default:
			// already had notification queued up; no need to handle it twice
		}
	})
}

func (bus *notificationsBus) handleReconnect() {
	// alert all listeners of connection break so they can check for things
	// they may have missed
	bus.notifications.each(func(sink chan bool) {
		select {
		case sink <- false:
			// notify that connection was lost, so listener can check for
			// things that may have changed while connection was lost
		default:
			// already had notification queued up; no need to check for
			// anything missed since something will be notified anyway
		}
	})
}

func newNotificationsMap() *notificationsMap {
	return &notificationsMap{
		notifications: map[string]map[chan bool]struct{}{},
	}
}

type notificationsMap struct {
	sync.RWMutex

	notifications map[string]map[chan bool]struct{}
}

func (m *notificationsMap) empty(channel string) bool {
	m.RLock()
	defer m.RUnlock()

	return len(m.notifications[channel]) == 0
}

func (m *notificationsMap) register(channel string, notify chan bool) {
	m.Lock()
	defer m.Unlock()

	sinks, found := m.notifications[channel]
	if !found {
		sinks = map[chan bool]struct{}{}
		m.notifications[channel] = sinks
	}

	sinks[notify] = struct{}{}
}

func (m *notificationsMap) unregister(channel string, notify chan bool) {
	m.Lock()
	defer m.Unlock()

	delete(m.notifications[channel], notify)
}

func (m *notificationsMap) each(f func(chan bool)) {
	m.RLock()
	defer m.RUnlock()

	for _, sinks := range m.notifications {
		for sink := range sinks {
			f(sink)
		}
	}
}

func (m *notificationsMap) eachForChannel(channel string, f func(chan bool)) {
	m.RLock()
	defer m.RUnlock()

	for sink := range m.notifications[channel] {
		f(sink)
	}
}
