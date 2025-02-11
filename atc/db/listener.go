package db

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// This is our own implementation of the Listener interface from when we used
// the lib/pq package. jackc/pgx does not provide a similar interface so we
// have to maintain our own now
type PgxListener struct {
	notify chan *pgconn.Notification

	lock       sync.Mutex
	conn       *pgx.Conn
	channels   map[string]struct{}
	cancelLock sync.Mutex
	cancelFunc context.CancelFunc
	opsDone    chan struct{}
}

var (
	listening struct{} //empty struct takes up zero memory
)

func NewPgxListener(conn *pgx.Conn) *PgxListener {
	l := &PgxListener{
		conn:     conn,
		notify:   make(chan *pgconn.Notification, 32),
		opsDone:  make(chan struct{}),
		channels: make(map[string]struct{}),
	}

	go l.ListenerMain()
	return l
}

func (l *PgxListener) Close() error {
	l.cancelNotificatonListener()
	l.lock.Lock()
	defer l.lock.Unlock()

	var err error
	if l.conn != nil {
		err = l.conn.Close(context.Background())
	}

	close(l.opsDone)
	return err
}

func (l *PgxListener) Listen(channel string) error {
	l.cancelNotificatonListener()
	l.lock.Lock()
	defer l.lock.Unlock()

	l.channels[channel] = listening
	_, err := l.conn.Exec(context.Background(), fmt.Sprintf("LISTEN %s", channel))
	l.opsDone <- struct{}{}
	return err
}

func (l *PgxListener) Unlisten(channel string) error {
	l.cancelNotificatonListener()
	l.lock.Lock()
	defer l.lock.Unlock()

	delete(l.channels, channel)
	_, err := l.conn.Exec(context.Background(), fmt.Sprintf("UNLISTEN %s", channel))
	l.opsDone <- struct{}{}
	return err
}

func (l *PgxListener) NotificationChannel() <-chan *pgconn.Notification {
	return l.notify
}

func (l *PgxListener) cancelNotificatonListener() {
	for {
		l.cancelLock.Lock()
		if l.cancelFunc != nil {
			l.cancelFunc()
			l.cancelFunc = nil
			l.cancelLock.Unlock()
			return
		}
		l.cancelLock.Unlock()
	}
}

func (l *PgxListener) listenerLoop() {
	for {
		if l.conn == nil || l.conn.IsClosed() {
			//connection was closed
			return
		}

		l.lock.Lock()
		l.cancelLock.Lock()
		ctx, cancelFunc := context.WithCancel(context.Background())
		l.cancelFunc = cancelFunc
		l.cancelLock.Unlock()
		notification, err := l.conn.WaitForNotification(ctx)
		l.lock.Unlock()

		if err != nil {
			if errors.Is(err, context.Canceled) {
				// Someone cancelled us, wait until they're done their work
				<-l.opsDone
				continue
			}
			if !pgconn.SafeToRetry(err) {
				// TODO: something has happened. Lets reset the connection and try again
			}
		}

		if notification != nil {
			l.notify <- notification
		}
	}
}

func (l *PgxListener) ListenerMain() {
	l.listenerLoop()
	close(l.notify)
}
