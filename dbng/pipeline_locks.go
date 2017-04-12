package dbng

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db/lock"
)

func (p *pipeline) AcquireResourceCheckingLockWithIntervalCheck(
	logger lager.Logger,
	resource Resource,
	resourceTypes atc.VersionedResourceTypes,
	interval time.Duration,
	immediate bool,
) (lock.Lock, bool, error) {
	tx, err := p.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer tx.Rollback()

	params := []interface{}{resource.Name(), p.id}

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

	resourceConfig, err := constructResourceConfig(tx, resource.Type(), resource.Source(), resourceTypes)
	if err != nil {
		return nil, false, err
	}

	return acquireResourceCheckingLock(
		logger.Session("lock", lager.Data{"resource": resource.Name()}),
		tx,
		ForResource(resource.ID()),
		resourceConfig,
		p.lockFactory,
	)
}
