package db

import (
	"database/sql"
	"sync"
	"time"

	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Lease

type Lease interface {
	Break()
}

type lease struct {
	pdb          *pipelineDB
	resourceName string
	logger       lager.Logger

	attemptSignFunc func(*sql.Tx) (sql.Result, error)

	breakChan chan struct{}
	running   *sync.WaitGroup
}

func (l *lease) AttemptSign(resourceName string, interval time.Duration) (bool, error) {
	tx, err := l.pdb.conn.Begin()
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
	l.breakChan = make(chan struct{})
	l.running = &sync.WaitGroup{}
	l.running.Add(1)

	go l.keepLeased(interval)
}

func (c *lease) Break() {
	close(c.breakChan)
	c.running.Wait()
}

func (l *lease) keepLeased(interval time.Duration) {
	defer l.running.Done()

	ticker := time.NewTicker(interval / 2)
	defer ticker.Stop()

dance:
	for {
		select {
		case <-ticker.C:
			renewed, err := l.AttemptSign(l.resourceName, interval)
			if err != nil {
				l.logger.Error("failed-to-renew-lease", err)
				break
			}

			if !renewed {
				l.logger.Debug("lease-was-already-renewed-recently")
				break
			}

			l.logger.Debug("renewed-the-lease")
		case <-l.breakChan:
			break dance
		}
	}
}
