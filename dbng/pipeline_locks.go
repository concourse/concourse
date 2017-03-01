package dbng

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db/lock"
)

func (p *pipeline) AcquireResourceCheckingLock(
	logger lager.Logger,
	resource *Resource,
	resourceTypes atc.ResourceTypes,
	interval time.Duration,
	immediate bool,
) (lock.Lock, bool, error) {
	tx, err := p.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	resourceConfig, err := constructResourceConfig(tx, resource.Type, resource.Source, resourceTypes, p.id)
	if err != nil {
		return nil, false, err
	}

	usedResourceConfig, err := ForResource{ResourceID: resource.ID}.UseResourceConfig(
		logger,
		tx,
		p.lockFactory,
		resourceConfig,
	)
	if err != nil {
		return nil, false, err
	}

	defer tx.Rollback()

	params := []interface{}{resource.Name, p.id}

	condition := ""
	if !immediate {
		condition = "AND now() - last_checked > ($3 || ' SECONDS')::INTERVAL"
		params = append(params, interval.Seconds())
	}

	updated, err := checkIfRowsUpdated(tx, `
		UPDATE resources
		SET last_checked = now()
		WHERE name = $1
			AND pipeline_id = $2
	`+condition, params...)
	if err != nil {
		return nil, false, err
	}

	if !updated {
		return nil, false, nil
	}

	lock := p.lockFactory.NewLock(
		logger.Session("lock", lager.Data{
			"resource": resource.Name,
		}),
		lock.NewResourceConfigCheckingLockID(usedResourceConfig.ID),
	)

	acquired, err := lock.Acquire()
	if err != nil {
		return nil, false, err
	}

	if !acquired {
		return nil, false, nil
	}

	err = tx.Commit()
	if err != nil {
		lock.Release()
		return nil, false, err
	}

	return lock, true, nil
}
