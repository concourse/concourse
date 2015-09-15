package db

import (
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

	breakChan chan struct{}
	running   *sync.WaitGroup
}

func (c *lease) AttemptSign(resourceName string, interval time.Duration) (bool, error) {
	tx, err := c.pdb.conn.Begin()
	if err != nil {
		return false, err
	}

	defer tx.Rollback()

	result, err := tx.Exec(`
		UPDATE resources
		SET last_checked = now()
		WHERE name = $1
			AND pipeline_id = $2
			AND now() - last_checked > ($3 || ' SECONDS')::INTERVAL
	`, resourceName, c.pdb.ID, interval.Seconds())
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

func (c *lease) KeepSigned(interval time.Duration) {
	c.breakChan = make(chan struct{})
	c.running = &sync.WaitGroup{}
	c.running.Add(1)

	go c.keepLeased(interval)
}

func (c *lease) Break() {
	close(c.breakChan)
	c.running.Wait()
}

func (c *lease) keepLeased(interval time.Duration) {
	defer c.running.Done()

	ticker := time.NewTicker(interval / 2)
	defer ticker.Stop()

dance:
	for {
		select {
		case <-ticker.C:
			renewed, err := c.AttemptSign(c.resourceName, interval)
			if err != nil {
				c.logger.Error("failed-to-renew-lease", err)
				break
			}

			if !renewed {
				c.logger.Debug("lease-was-already-renewed-recently")
				break
			}

			c.logger.Debug("renewed-the-lease")
		case <-c.breakChan:
			break dance
		}
	}
}
