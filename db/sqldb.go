package db

import (
	"fmt"
	"time"

	"github.com/concourse/atc/db/lock"
)

type SQLDB struct {
	conn        Conn
	lockFactory lock.LockFactory
	bus         *notificationsBus

	buildFactory *buildFactory
}

func NewSQL(
	sqldbConnection Conn,
	bus *notificationsBus,
	lockFactory lock.LockFactory,
) *SQLDB {
	return &SQLDB{
		conn:         sqldbConnection,
		lockFactory:  lockFactory,
		bus:          bus,
		buildFactory: newBuildFactory(sqldbConnection, bus, lockFactory),
	}
}

type nonOneRowAffectedError struct {
	RowsAffected int64
}

func (err nonOneRowAffectedError) Error() string {
	return fmt.Sprintf("expected 1 row to be updated; got %d", err.RowsAffected)
}

type scannable interface {
	Scan(destinations ...interface{}) error
}

type conditionNotifier struct {
	cond func() (bool, error)

	bus     *notificationsBus
	channel string

	notified chan bool
	notify   chan struct{}

	stop chan struct{}
}

func (notifier *conditionNotifier) Notify() <-chan struct{} {
	return notifier.notify
}

func (notifier *conditionNotifier) Close() error {
	close(notifier.stop)
	return notifier.bus.Unlisten(notifier.channel, notifier.notified)
}

func (notifier *conditionNotifier) watch() {
	for {
		c, err := notifier.cond()
		if err != nil {
			select {
			case <-time.After(5 * time.Second):
				continue
			case <-notifier.stop:
				return
			}
		}

		if c {
			notifier.sendNotification()
		}

	dance:
		for {
			select {
			case <-notifier.stop:
				return
			case ok := <-notifier.notified:
				if ok {
					notifier.sendNotification()
				} else {
					break dance
				}
			}
		}
	}
}

func (notifier *conditionNotifier) sendNotification() {
	select {
	case notifier.notify <- struct{}{}:
	default:
	}
}
