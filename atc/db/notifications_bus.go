package db

import (
	"database/sql"
	"sync"

	"github.com/lib/pq"
)

type NotificationQueueMode bool

const (
	DontQueueNotifications NotificationQueueMode = false
	QueueNotifications     NotificationQueueMode = true
)

type Notification struct {
	Payload string
	Healthy bool
}

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
	Listen(channel string, queueMode NotificationQueueMode) (chan Notification, error)
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

func (bus *notificationsBus) Listen(channel string, queueMode NotificationQueueMode) (chan Notification, error) {
	bus.Lock()
	defer bus.Unlock()

	if bus.notifications.empty(channel) {
		err := bus.listener.Listen(channel)
		if err != nil {
			return nil, err
		}
	}

	chanSize := 1
	if queueMode == QueueNotifications {
		// 32 should be sufficient most of the time
		// but when the consumer is too slow, we start to fill up `q`
		chanSize = 32
	}

	notify := make(chan Notification, chanSize)

	var q *queue
	if queueMode == QueueNotifications {
		q = newQueue()
		go q.drain(notify)
	}
	bus.notifications.register(channel, notify, q)
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
	bus.notifications.eachForChannel(notification.Channel, func(sink chan Notification, q *queue) {
		n := Notification{Healthy: true, Payload: notification.Extra}
		select {
		case sink <- n:
			// notified of message being received (or queued up)
		default:
			if q == nil {
				// already had notification queued up; no need to handle it twice
				return
			}
			q.enqueue(n)
		}
	})
}

func (bus *notificationsBus) handleReconnect() {
	// alert all listeners of connection break so they can check for things
	// they may have missed
	bus.notifications.each(func(sink chan Notification, q *queue) {
		n := Notification{Healthy: false}
		select {
		case sink <- n:
			// notify that connection was lost, so listener can check for
			// things that may have changed while connection was lost
		default:
			if q == nil {
				// already had notification queued up; no need to check for
				// anything missed since something will be notified anyway
				return
			}
			q.enqueue(n)
		}
	})
}

type queue struct {
	sync.Mutex
	dirty  chan struct{}
	closed chan struct{}
	queue  []Notification
}

func newQueue() *queue {
	return &queue{dirty: make(chan struct{}, 1)}
}

func (q *queue) enqueue(n Notification) {
	q.Lock()
	q.queue = append(q.queue, n)
	q.Unlock()

	select {
	case q.dirty <- struct{}{}:
	default:
		// It was already dirty
	}
}

func (q *queue) drain(notify chan Notification) {
	for {
		select {
		case <-q.closed:
			return
		case <-q.dirty:
		}
		q.Lock()
		clone := make([]Notification, len(q.queue))
		copy(clone, q.queue)
		q.queue = q.queue[:0]
		q.Unlock()

		for _, elem := range clone {
			select {
			case <-q.closed:
				return
			case notify <- elem:
			}
		}
	}
}

func newNotificationsMap() *notificationsMap {
	return &notificationsMap{
		notifications: make(map[string]map[chan Notification]*queue),
	}
}

type notificationsMap struct {
	sync.RWMutex

	notifications map[string]map[chan Notification]*queue
}

func (m *notificationsMap) empty(channel string) bool {
	m.RLock()
	defer m.RUnlock()

	return len(m.notifications[channel]) == 0
}

func (m *notificationsMap) register(channel string, notify chan Notification, q *queue) {
	m.Lock()
	defer m.Unlock()

	sinks, found := m.notifications[channel]
	if !found {
		sinks = make(map[chan Notification]*queue)
		m.notifications[channel] = sinks
	}

	sinks[notify] = q
}

func (m *notificationsMap) unregister(channel string, notify chan Notification) {
	m.Lock()
	defer m.Unlock()

	n, ok := m.notifications[channel]
	if !ok {
		return
	}
	q := n[notify]
	if q != nil {
		close(q.closed)
	}
	delete(m.notifications[channel], notify)
}

func (m *notificationsMap) each(f func(chan Notification, *queue)) {
	m.RLock()
	defer m.RUnlock()

	for _, sinks := range m.notifications {
		for sink, q := range sinks {
			f(sink, q)
		}
	}
}

func (m *notificationsMap) eachForChannel(channel string, f func(chan Notification, *queue)) {
	m.RLock()
	defer m.RUnlock()

	for sink, q := range m.notifications[channel] {
		f(sink, q)
	}
}
