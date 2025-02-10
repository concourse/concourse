package db

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// This is our own implementation of the Listener interface from when we used
// the lib/pq package. jackc/pgx does not provide a similar interface so we
// have to maintain our own now
type pgxListener struct {
	Notify chan *pgconn.Notification

	lock       sync.Mutex
	conn       *pgx.Conn
	cancelFunc context.CancelFunc
	//TODO: add a channel to track channels we're listening to so we don't issue the command twice
}

func NewPgxListener(conn *pgx.Conn) *pgxListener {
	l := &pgxListener{
		conn:   conn,
		Notify: make(chan *pgconn.Notification, 32),
	}

	go l.ListenerMain()
	return l
}

func (l *pgxListener) Close() error {
	if l.cancelFunc != nil {
		l.cancelFunc()
	}

	l.lock.Lock()
	defer l.lock.Unlock()

	if l.conn != nil {
		return l.conn.Close(context.Background())
	}

	return nil
}

func (l *pgxListener) Listen(channel string) error {
	if l.cancelFunc != nil {
		l.cancelFunc()
	}

	l.lock.Lock()
	defer l.lock.Unlock()

	_, err := l.conn.Exec(context.Background(), fmt.Sprintf("LISTEN %s", channel))
	return err
}

func (l *pgxListener) Unlisten(channel string) error {
	if l.cancelFunc != nil {
		l.cancelFunc()
	}

	l.lock.Lock()
	defer l.lock.Unlock()

	_, err := l.conn.Exec(context.Background(), fmt.Sprintf("UNLISTEN %s", channel))
	return err
}

func (l *pgxListener) NotificationChannel() <-chan *pgconn.Notification {
	return l.Notify
}

func (l *pgxListener) listenerLoop() {
	for {
		if l.conn == nil || l.conn.IsClosed() {
			//connection was closed and cleared out
			return
		}

		l.lock.Lock()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		l.cancelFunc = cancel
		notification, err := l.conn.WaitForNotification(ctx)
		l.lock.Unlock()
		l.cancelFunc = nil

		if err != nil {
			if errors.Is(err, context.Canceled) {
				// Someone cancelled us, give them time to grab the lock
				time.Sleep(10 * time.Millisecond)
				continue
			}
		}

		if notification != nil {
			l.Notify <- notification
		}
	}
}

func (l *pgxListener) ListenerMain() {
	l.listenerLoop()
	close(l.Notify)
}
