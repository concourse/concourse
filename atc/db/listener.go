package db

import (
	"context"
	"fmt"

	"github.com/cenkalti/backoff/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This is our own implementation of the Listener interface from when we used
// the lib/pq package. jackc/pgx does not provide a similar interface so we
// have to maintain our own now
type PgxListener struct {
	notify chan *pgconn.Notification

	pool       *pgxpool.Pool
	conn       *pgxpool.Conn
	channels   map[string]struct{}
	cancelFunc context.CancelFunc
	comms      chan struct{}
}

var (
	listening     struct{}
	askingForTurn struct{}
	itsYourTurn   struct{}
	start         struct{}
)

func NewPgxListener(pool *pgxpool.Pool) *PgxListener {
	conn, err := pool.Acquire(context.Background())
	if err != nil {
		panic(err)
	}

	l := &PgxListener{
		pool:     pool,
		conn:     conn,
		notify:   make(chan *pgconn.Notification, 32),
		comms:    make(chan struct{}),
		channels: make(map[string]struct{}),
	}

	go l.ListenerMain()
	return l
}

func (l *PgxListener) Close() error {
	l.comms <- askingForTurn
	<-l.comms

	if l.conn != nil {
		l.conn.Release()
		l.conn = nil
		// Don't need to manually close the pool
	}

	close(l.comms)
	return nil
}

func (l *PgxListener) Listen(channel string) error {
	l.comms <- askingForTurn
	<-l.comms

	l.channels[channel] = listening
	_, err := l.conn.Exec(context.Background(), fmt.Sprintf("LISTEN %s", channel))
	l.comms <- itsYourTurn
	return err
}

func (l *PgxListener) Unlisten(channel string) error {
	l.comms <- askingForTurn
	<-l.comms

	delete(l.channels, channel)
	_, err := l.conn.Exec(context.Background(), fmt.Sprintf("UNLISTEN %s", channel))
	l.comms <- itsYourTurn
	return err
}

func (l *PgxListener) NotificationChannel() <-chan *pgconn.Notification {
	return l.notify
}

func (l *PgxListener) listenerLoop() {
	var (
		notifyDone  = make(chan struct{})
		stillMyTurn = true

		notification *pgconn.Notification
		err          error
	)

	for {
		if !stillMyTurn {
			<-l.comms
			stillMyTurn = true
		}

		if l.conn == nil {
			return
		}

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			notification, err = l.conn.Conn().WaitForNotification(ctx)
			if notification != nil {
				l.notify <- notification
			}
			notifyDone <- itsYourTurn
		}()

		select {
		case <-notifyDone:
			if err != nil {
				// Will retry to a max of 15mins
				_, err = backoff.Retry(ctx, l.reconnect, backoff.WithBackOff(backoff.NewExponentialBackOff()))
				if err != nil {
					panic(fmt.Errorf("unable to reconnect to the database: %w", err))
				}

				//listen to all channels again
				for channel, _ := range l.channels {
					l.conn.Exec(ctx, fmt.Sprintf("LISTEN %s", channel))
				}
			}
			continue
		case t, ok := <-l.comms:
			cancel()
			<-notifyDone
			if ok && t == askingForTurn {
				stillMyTurn = false
				l.comms <- itsYourTurn
				continue
			}
			stillMyTurn = true
			continue
		}
	}
}

func (l *PgxListener) ListenerMain() {
	l.listenerLoop()
	close(l.notify)
}

func (l *PgxListener) reconnect() (bool, error) {
	ctx := context.Background()
	l.conn.Release()
	l.pool.Reset()

	newConn, err := l.pool.Acquire(ctx)
	if err != nil {
		return false, err
	}

	l.conn = newConn
	err = l.conn.Ping(ctx)
	if err != nil {
		return false, err
	}

	return true, nil
}
