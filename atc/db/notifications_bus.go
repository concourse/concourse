package db

import (
	"database/sql"
	"sync"

	"github.com/lib/pq"
)

type NotificationsBus interface {
	Notify(channel string) error
	Listen(channel string) (chan bool, error)
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

	go bus.wait()

	return bus
}

func (bus *notificationsBus) Close() error {
	return bus.listener.Close()
}

func (bus *notificationsBus) Notify(channel string) error {
	_, err := bus.conn.Exec("NOTIFY " + channel)
	return err
}

func (bus *notificationsBus) Listen(channel string) (chan bool, error) {

	bus.notificationsL.Lock()
	defer bus.notificationsL.Unlock()

	if len(bus.notifications[channel]) == 0 {
		err := bus.listener.Listen(channel)
		if err != nil {
			return nil, err
		}
	}

	notify := make(chan bool, 1)

	sinks, found := bus.notifications[channel]
	if !found {
		sinks = map[chan bool]struct{}{}
		bus.notifications[channel] = sinks
	}

	sinks[notify] = struct{}{}

	return notify, nil
}

func (bus *notificationsBus) Unlisten(channel string, notify chan bool) error {
	bus.notificationsL.Lock()
	defer bus.notificationsL.Unlock()

	delete(bus.notifications[channel], notify)

	if len(bus.notifications[channel]) == 0 {
		return bus.listener.Unlisten(channel)
	}

	return nil
}

func (bus *notificationsBus) wait() {
	for {

		notification, ok := <-bus.listener.Notify
		if !ok {
			break
		}

		bus.notificationsL.Lock()

		if notification != nil {
			for sink := range bus.notifications[notification.Channel] {
				sink <- true
			}
		}

		bus.notificationsL.Unlock()
	}
}
