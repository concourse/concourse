package db

import (
	"database/sql"
	"errors"
	"time"

	"github.com/concourse/concourse/atc/db/lock"
	"github.com/lib/pq"
)

type NotificationsBus interface {
	Notify(channel string) error
	Listen(channel string) (chan bool, error)
	Unlisten(channel string, notify chan bool) error
	Close() error
}

type Mutex interface {
	Unlock()
	Lock() bool
	LockWithTimeout(timeout time.Duration) bool
}

type notificationsBus struct {
	listener *pq.Listener
	conn     *sql.DB

	notifications  map[string]map[chan bool]struct{}
	notificationsL Mutex
}

func NewNotificationsBus(listener *pq.Listener, conn *sql.DB, timeout time.Duration) NotificationsBus {
	bus := &notificationsBus{
		listener: listener,
		conn:     conn,

		notifications:  make(map[string]map[chan bool]struct{}),
		notificationsL: lock.NewMutex(timeout),
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
	if bus.notificationsL.Lock() {
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

	return nil, errors.New("timeout")
}

func (bus *notificationsBus) Unlisten(channel string, notify chan bool) error {
	if bus.notificationsL.Lock() {
		defer bus.notificationsL.Unlock()

		delete(bus.notifications[channel], notify)

		if len(bus.notifications[channel]) == 0 {
			return bus.listener.Unlisten(channel)
		}

		return nil
	}

	return errors.New("timeout")
}

func (bus *notificationsBus) wait() {
	for {

		notification, ok := <-bus.listener.Notify
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
	if bus.notificationsL.Lock() {
		defer bus.notificationsL.Unlock()

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
	}
}

func (bus *notificationsBus) handleReconnect() {
	if bus.notificationsL.Lock() {
		defer bus.notificationsL.Unlock()

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
}
