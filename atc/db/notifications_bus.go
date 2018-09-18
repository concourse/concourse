package db

import (
	"database/sql"
	"sync"

	"github.com/lib/pq"
)

type NotificationsBus interface {
	Listen(channel string) (chan bool, error)
	Notify(channel string) error
	Unlisten(channel string, notify chan bool) error
	Close() error
}

type notificationsBus struct {
	listener *pq.Listener
	conn     *sql.DB

	notifications  map[string]map[chan bool]struct{}
	notificationsL sync.Mutex
}

func NewNotificationsBus(listener *pq.Listener, conn *sql.DB) NotificationsBus {
	bus := &notificationsBus{
		listener: listener,
		conn:     conn,

		notifications: make(map[string]map[chan bool]struct{}),
	}

	go bus.dispatchNotifications()

	return bus
}

func (bus *notificationsBus) Close() error {
	return bus.listener.Close()
}

func (bus *notificationsBus) Listen(channel string) (chan bool, error) {
	bus.notificationsL.Lock()
	firstListen := len(bus.notifications[channel]) == 0

	if firstListen {
		err := bus.listener.Listen(channel)
		if err != nil {
			bus.notificationsL.Unlock()
			return nil, err
		}
	}

	// buffer so that notifications can be nonblocking (only need one at a time)
	notify := make(chan bool, 1)

	sinks, found := bus.notifications[channel]
	if !found {
		sinks = map[chan bool]struct{}{}
		bus.notifications[channel] = sinks
	}

	sinks[notify] = struct{}{}

	bus.notificationsL.Unlock()

	return notify, nil
}

func (bus *notificationsBus) Notify(channel string) error {
	_, err := bus.conn.Exec("NOTIFY " + channel)
	return err
}

func (bus *notificationsBus) Unlisten(channel string, notify chan bool) error {
	bus.notificationsL.Lock()
	delete(bus.notifications[channel], notify)
	lastSink := len(bus.notifications[channel]) == 0
	bus.notificationsL.Unlock()

	if lastSink {
		return bus.listener.Unlisten(channel)
	}

	return nil
}

func (bus *notificationsBus) dispatchNotifications() {
	for {
		notification, ok := <-bus.listener.Notify
		if !ok {
			break
		}

		gotNotification := notification != nil

		bus.notificationsL.Lock()

		if gotNotification {
			// alert any relevant listeners of notification being received
			// (nonblocking)
			for sink := range bus.notifications[notification.Channel] {
				select {
				case sink <- true:
					// notified of message being received (or queued up)
				default:
					// already had notification queued up; no need to handle it twice
				}
			}
		} else {
			// alert all listeners of connection break so they can check for things
			// they may have missed
			for _, sinks := range bus.notifications {
				for sink := range sinks {
					select {
					case sink <- false:
						// notify that connection was lost, so listener can check for
						// things that may have changed while connection was lost
					default:
						// already had notification queued up; no need to check for
						// anything missed since something will be notified anyway
					}
				}
			}
		}

		bus.notificationsL.Unlock()
	}
}
