package db

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/cenkalti/backoff/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This is our own implementation of the Listener interface from when we used
// the lib/pq package. jackc/pgx does not provide a similar interface so we
// have to maintain our own now
type PgxListener struct {
	notify chan *pgconn.Notification

	lock       sync.Mutex
	pool       *pgxpool.Pool
	conn       *pgx.Conn
	channels   map[string]struct{}
	cancelLock sync.Mutex
	cancelFunc context.CancelFunc
	opsDone    chan struct{}
}

var (
	listening struct{} //empty struct takes up zero memory
)

func NewPgxListener(pool *pgxpool.Pool) *PgxListener {
	conn, err := pool.Acquire(context.Background())
	if err != nil {
		panic(err)
	}

	l := &PgxListener{
		pool:     pool,
		conn:     conn.Conn(),
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

	if l.conn != nil {
		l.conn.Close(context.Background())
		// Don't need to manually close the pool
	}

	close(l.opsDone)
	return nil
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
				// Something bad has happened, let's try recovering
				l.lock.Lock()

				ctx := context.Background()
				reconnect := func() (bool, error) {
					l.conn.Close(ctx)
					l.pool.Reset()

					newConn, err := l.pool.Acquire(ctx)
					if err != nil {
						return false, err
					}

					l.conn = newConn.Conn()
					err = l.conn.Ping(ctx)
					if err != nil {
						return false, err
					}

					return true, nil
				}

				// Will retry to a max of 15mins
				_, err := backoff.Retry(ctx, reconnect, backoff.WithBackOff(backoff.NewExponentialBackOff()))
				if err != nil {
					panic(fmt.Errorf("unable to reconnect to the database: %w", err))
				}

				//listen to all channels again
				for channel, _ := range l.channels {
					l.conn.Exec(ctx, fmt.Sprintf("LISTEN %s", channel))
				}

				l.lock.Unlock()
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
