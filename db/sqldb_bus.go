package db

import (
	"sync"

	"github.com/lib/pq"
)

type notificationsBus struct {
	listener *pq.Listener

	notifications  map[string]map[chan bool]struct{}
	notificationsL sync.Mutex
}

func newNotificationsBus(listener *pq.Listener) *notificationsBus {
	bus := &notificationsBus{
		listener: listener,

		notifications: make(map[string]map[chan bool]struct{}),
	}

	go bus.dispatchNotifications()

	return bus
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

		for sink, _ := range bus.notifications[notification.Channel] {
			select {
			case sink <- gotNotification:
			default:
			}
		}

		bus.notificationsL.Unlock()
	}
}
