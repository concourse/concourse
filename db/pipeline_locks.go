package db

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/atc/db/lock"
)

func (p *pipeline) AcquireResourceCheckingLockWithIntervalCheck(
	logger lager.Logger,
	resource Resource,
	interval time.Duration,
	immediate bool,
	variablesSource template.Variables,
) (lock.Lock, bool, error) {
	resourceTypes, err := p.ResourceTypes()
	if err != nil {
		logger.Error("failed-to-get-resource-types", err)
		return nil, false, err
	}

	evaluatedSource, err := resource.EvaluatedSource(variablesSource)
	if err != nil {
		logger.Error("failed-to-evaluate-resource-source", err)
		return nil, false, err
	}

	resourceConfig, err := constructResourceConfig(resource.Type(), evaluatedSource, resourceTypes.Deserialize())
	if err != nil {
		return nil, false, err
	}

	lock, acquired, err := acquireResourceCheckingLock(
		logger.Session("lock", lager.Data{"resource": resource.Name()}),
		p.conn,
		ForResource(resource.ID()),
		resourceConfig,
		p.lockFactory,
	)

	if err != nil {
		return nil, false, err
	}

	if !acquired {
		return nil, false, nil
	}

	intervalUpdated, err := p.checkIfResourceIntervalUpdated(resource.Name(), interval, immediate)
	if err != nil {
		lock.Release()
		return nil, false, err
	}

	if !intervalUpdated {
		lock.Release()
		return nil, false, nil
	}

	return lock, true, nil
}

func (p *pipeline) AcquireResourceTypeCheckingLockWithIntervalCheck(
	logger lager.Logger,
	resourceTypeName string,
	interval time.Duration,
	immediate bool,
) (lock.Lock, bool, error) {
	resourceType, found, err := p.ResourceType(resourceTypeName)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, ResourceTypeNotFoundError{Name: resourceTypeName}
	}

	resourceTypes, err := p.ResourceTypes()
	if err != nil {
		logger.Error("failed-to-get-resource-types", err)
		return nil, false, err
	}

	deserializedResourceTypes := resourceTypes.Deserialize().Without(resourceTypeName)

	resourceConfig, err := constructResourceConfig(resourceType.Type(), resourceType.Source(), deserializedResourceTypes)
	if err != nil {
		return nil, false, err
	}

	lock, acquired, err := acquireResourceCheckingLock(
		logger.Session("lock", lager.Data{"resource-type": resourceTypeName}),
		p.conn,
		ForResourceType(resourceType.ID()),
		resourceConfig,
		p.lockFactory,
	)
	if err != nil {
		return nil, false, err
	}

	if !acquired {
		return nil, false, nil
	}

	intervalUpdated, err := p.checkIfResourceTypeIntervalUpdated(resourceTypeName, interval, immediate)
	if err != nil {
		lock.Release()
		return nil, false, err
	}

	if !intervalUpdated {
		lock.Release()
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

	defer tx.Rollback()

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

	defer tx.Rollback()

	params := []interface{}{resourceName, p.id}

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
