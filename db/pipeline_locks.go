package db

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db/lock"
)

func (p *pipeline) AcquireResourceCheckingLockWithIntervalCheck(
	logger lager.Logger,
	resourceName string,
	usedResourceConfig *UsedResourceConfig,
	interval time.Duration,
	immediate bool,
) (lock.Lock, bool, error) {
	lock, acquired, err := p.lockFactory.Acquire(
		logger,
		lock.NewResourceConfigCheckingLockID(usedResourceConfig.ID),
	)
	if err != nil {
		return nil, false, err
	}

	if !acquired {
		return nil, false, nil
	}

	intervalUpdated, err := p.checkIfResourceIntervalUpdated(resourceName, interval, immediate)
	if err != nil {
		lockErr := lock.Release()
		if lockErr != nil {
			logger.Fatal("failed-to-release-lock", lockErr)
		}
		return nil, false, err
	}

	if !intervalUpdated {
		lockErr := lock.Release()
		if lockErr != nil {
			logger.Fatal("failed-to-release-lock", lockErr)
		}
		return nil, false, nil
	}

	return lock, true, nil
}

func (p *pipeline) AcquireResourceTypeCheckingLockWithIntervalCheck(
	logger lager.Logger,
	resourceTypeName string,
	usedResourceConfig *UsedResourceConfig,
	interval time.Duration,
	immediate bool,
) (lock.Lock, bool, error) {
	lock, acquired, err := p.lockFactory.Acquire(
		logger,
		lock.NewResourceConfigCheckingLockID(usedResourceConfig.ID),
	)
	if err != nil {
		return nil, false, err
	}

	if !acquired {
		return nil, false, nil
	}

	intervalUpdated, err := p.checkIfResourceTypeIntervalUpdated(resourceTypeName, interval, immediate)
	if err != nil {

		return nil, false, err
	}

	if !intervalUpdated {
		lockErr := lock.Release()
		if lockErr != nil {
			logger.Fatal("failed-to-release-lock", lockErr)
		}
		return nil, false, nil
	}

	return lock, true, nil
}

func (p *pipeline) checkIfResourceTypeIntervalUpdated(
	resourceTypeName string,
	interval time.Duration,
	immediate bool,
) (bool, error) {
	tx, err := p.conn.Begin()
	if err != nil {
		return false, err
	}

	defer Rollback(tx)

	params := []interface{}{resourceTypeName, p.id}

	condition := ""
	if !immediate {
		condition = "AND now() - last_checked > ($3 || ' SECONDS')::INTERVAL"
		params = append(params, interval.Seconds())
	}

	updated, err := checkIfRowsUpdated(tx, `
			UPDATE resource_types
			SET last_checked = now()
			WHERE name = $1
				AND pipeline_id = $2
		`+condition, params...)
	if err != nil {
		return false, err
	}

	if !updated {
		return false, nil
	}

	err = tx.Commit()
	if err != nil {
		return false, err
	}

	return true, nil
}

func (p *pipeline) checkIfResourceIntervalUpdated(
	resourceName string,
	interval time.Duration,
	immediate bool,
) (bool, error) {
	tx, err := p.conn.Begin()
	if err != nil {
		return false, err
	}

	defer Rollback(tx)

	params := []interface{}{resourceName, p.id}

	condition := ""
	if !immediate {
		condition = "AND (now() - last_checked > ($3 || ' SECONDS')::INTERVAL OR last_checked IS NULL)"
		params = append(params, interval.Seconds())
	}

	updated, err := checkIfRowsUpdated(tx, `
			UPDATE resources
			SET last_checked = now()
			WHERE name = $1
				AND pipeline_id = $2
		`+condition, params...)
	if err != nil {
		return false, err
	}

	if !updated {
		return false, nil
	}

	err = tx.Commit()
	if err != nil {
		return false, err
	}

	return true, nil
}
