package db

import (
	"database/sql"
	"sync"

	"github.com/lib/pq"
)

type Notification struct {
	Payload string
	Healthy bool
}

//counterfeiter:generate . Listener

type Listener interface {
	Close() error
	Listen(channel string) error
	Unlisten(channel string) error
	NotificationChannel() <-chan *pq.Notification
}

//counterfeiter:generate . Executor
type Executor interface {
	Exec(statement string, args ...interface{}) (sql.Result, error)
}

type NotificationsBus interface {
	Notify(channel string) error
	Listen(channel string, queueSize int) (chan Notification, error)
	Unlisten(channel string, notify chan Notification) error
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

func (bus *notificationsBus) Listen(channel string, queueSize int) (chan Notification, error) {
	bus.Lock()
	defer bus.Unlock()

	if bus.notifications.empty(channel) {
		err := bus.listener.Listen(channel)
		if err != nil {
			return nil, err
		}
	}

	notify := make(chan Notification, queueSize)
	bus.notifications.register(channel, notify)
	return notify, nil
}

func (bus *notificationsBus) Unlisten(channel string, notify chan Notification) error {
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
	bus.notifications.eachForChannel(notification.Channel, func(sink chan Notification) {
		n := Notification{Healthy: true, Payload: notification.Extra}
		select {
		case sink <- n:
			// notified of message being received (or queued up)
		default:
			// queue overflowed - just ignore
		}
	})
}

func (bus *notificationsBus) handleReconnect() {
	// alert all listeners of connection break so they can check for things
	// they may have missed
	bus.notifications.each(func(sink chan Notification) {
		n := Notification{Healthy: false}
		select {
		case sink <- n:
			// notify that connection was lost, so listener can check for
			// things that may have changed while connection was lost
		default:
			// queue overflowed - just ignore
		}
	})
}

func newNotificationsMap() *notificationsMap {
	return &notificationsMap{
		notifications: make(map[string]map[chan Notification]struct{}),
	}
}

type notificationsMap struct {
	sync.RWMutex

	notifications map[string]map[chan Notification]struct{}
}

func (m *notificationsMap) empty(channel string) bool {
	m.RLock()
	defer m.RUnlock()

	return len(m.notifications[channel]) == 0
}

func (m *notificationsMap) register(channel string, notify chan Notification) {
	m.Lock()
	defer m.Unlock()

	sinks, found := m.notifications[channel]
	if !found {
		sinks = make(map[chan Notification]struct{})
		m.notifications[channel] = sinks
	}

	sinks[notify] = struct{}{}
}

func (m *notificationsMap) unregister(channel string, notify chan Notification) {
	m.Lock()
	defer m.Unlock()

	_, ok := m.notifications[channel]
	if !ok {
		return
	}
	delete(m.notifications[channel], notify)

	// Note: we don't call empty since we already acquired the lock.
	if len(m.notifications[channel]) == 0 {
		delete(m.notifications, channel)
	}
}

func (m *notificationsMap) each(f func(chan Notification)) {
	m.RLock()
	defer m.RUnlock()

	for _, sinks := range m.notifications {
		for sink := range sinks {
			f(sink)
		}
	}
}

func (m *notificationsMap) eachForChannel(channel string, f func(chan Notification)) {
	m.RLock()
	defer m.RUnlock()

	for sink := range m.notifications[channel] {
		f(sink)
	}
}
