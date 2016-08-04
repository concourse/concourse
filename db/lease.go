package db

import (
	"database/sql"
	"errors"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Lease

type Lease interface {
	Break()
}

type lease struct {
	conn   Conn
	logger lager.Logger

	attemptSignFunc func(Tx) (sql.Result, error)
	heartbeatFunc   func(Tx) (sql.Result, error)
	breakFunc       func()

	medalChan chan struct{}
	breakChan chan struct{}
	running   *sync.WaitGroup
}

//go:generate counterfeiter . SqlResult

type SqlResult interface {
	LastInsertId() (int64, error)
	RowsAffected() (int64, error)
}

//go:generate counterfeiter . LeaseTester

type LeaseTester interface {
	AttemptSign(Tx) (sql.Result, error)
	Heartbeat(Tx) (sql.Result, error)
	Break()
}

func NewLeaseForTesting(conn Conn, logger lager.Logger, leaseTester LeaseTester, interval time.Duration) (Lease, bool, error) {
	lease := &lease{
		conn:            conn,
		logger:          logger,
		attemptSignFunc: func(tx Tx) (sql.Result, error) { return leaseTester.AttemptSign(tx) },
		heartbeatFunc:   func(tx Tx) (sql.Result, error) { return leaseTester.Heartbeat(tx) },
		breakFunc:       func() { leaseTester.Break() },
	}

	renewed, err := lease.AttemptSign(interval)
	if err != nil {
		return nil, false, err
	}

	if !renewed {
		return nil, renewed, nil
	}

	lease.KeepSigned(interval)

	return lease, true, nil
}

func (l *lease) AttemptSign(interval time.Duration) (bool, error) {
	tx, err := l.conn.Begin()
	if err != nil {
		return false, err
	}

	defer tx.Rollback()

	result, err := l.attemptSignFunc(tx)
	if err != nil {
		return false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	if rows == 0 {
		return false, nil
	}

	err = tx.Commit()
	if err != nil {
		return false, err
	}

	return true, nil
}

func (l *lease) KeepSigned(interval time.Duration) {
	l.medalChan = make(chan struct{}, 1)
	l.medalChan <- struct{}{}

	l.breakChan = make(chan struct{})

	l.running = &sync.WaitGroup{}
	l.running.Add(1)

	go l.keepLeased(interval)
}

func (l *lease) Break() {
	if l.medalChan == nil {
		return
	}

	select {
	// the first thread to call break gets a gold medal
	case <-l.medalChan:
		close(l.breakChan)
		l.running.Wait()
		if l.breakFunc != nil {
			l.breakFunc()
		}
	default:
	}
}

func (l *lease) extend() error {
	tx, err := l.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	result, err := l.heartbeatFunc(tx)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return errors.New("lease not found")
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (l *lease) keepLeased(interval time.Duration) {
	defer l.running.Done()

	ticker := time.NewTicker(interval / 2)
	defer ticker.Stop()

dance:
	for {
		select {
		case <-ticker.C:
			err := l.extend()
			if err != nil {
				l.logger.Error("failed-to-renew-lease", err)
				break dance
			}

			l.logger.Debug("renewed-the-lease")
		case <-l.breakChan:
			break dance
		}
	}
}
