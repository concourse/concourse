package db

import (
	"sync"
	"time"

	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Contract

type Contract interface {
	Break()
}

type contract struct {
	pdb          *pipelineDB
	resourceName string
	logger       lager.Logger

	breakChan chan struct{}
	running   *sync.WaitGroup
}

func (c *contract) Sign(interval time.Duration) {
	c.breakChan = make(chan struct{})
	c.running = &sync.WaitGroup{}
	c.running.Add(1)

	go c.keepLeased(interval)
}

func (c *contract) Break() {
	close(c.breakChan)
	c.running.Wait()
}

func (c *contract) keepLeased(interval time.Duration) {
	defer c.running.Done()

	ticker := time.NewTicker(interval / 2)
	defer ticker.Stop()

dance:
	for {
		select {
		case <-ticker.C:
			renewed, err := c.pdb.renewLease(c.resourceName, interval)
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
