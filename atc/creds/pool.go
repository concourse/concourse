package creds

import (
	"sync"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"encoding/json"
)

//go:generate counterfeiter . VarSourcePool

type VarSourcePool interface {
	FindOrCreate(lager.Logger, string, map[string]interface{}, ManagerFactory) (Secrets, error)
	Size() int
}

type inPoolManager struct {
	varSourceName  string
	manager        Manager
	secretsFactory SecretsFactory
	lastUseTime    time.Time
	clock          clock.Clock
}

func (m *inPoolManager) Close(logger lager.Logger) {
	m.manager.Close(logger)
}

func (m *inPoolManager) NewSecrets() Secrets {
	m.lastUseTime = m.clock.Now()
	return NewNamedSecrets(m.secretsFactory.NewSecrets(), m.varSourceName)
}

type varSourcePool struct {
	pool  map[string]*inPoolManager
	lock  sync.Mutex
	ttl   time.Duration
	clock clock.Clock
}

func (pool *varSourcePool) Size() int {
	return len(pool.pool)
}

func (pool *varSourcePool) FindOrCreate(logger lager.Logger, varSourceName string, config map[string]interface{}, factory ManagerFactory) (Secrets, error) {
	b, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	key := varSourceName + string(b)

	pool.lock.Lock()
	defer pool.lock.Unlock()

	if _, ok := pool.pool[key]; !ok {
		manager, err := factory.NewInstance(config)
		if err != nil {
			return nil, err
		}
		err = manager.Init(logger)
		if err != nil {
			return nil, err
		}
		secretsFactory, err := manager.NewSecretsFactory(logger)
		if err != nil {
			return nil, err
		}

		pool.pool[key] = &inPoolManager{
			varSourceName:  varSourceName,
			clock:          pool.clock,
			manager:        manager,
			secretsFactory: secretsFactory,
		}
	} else {
		logger.Debug("found-existing-credential-manager")
	}

	return pool.pool[key].NewSecrets(), nil
}

func (pool *varSourcePool) Collect(logger lager.Logger) error {
	pool.lock.Lock()
	defer pool.lock.Unlock()

	logger.Debug("before-collect", lager.Data{"pool-size": len(pool.pool)})

	toDeleteKeys := []string{}
	for key, manager := range pool.pool {
		if manager.lastUseTime.Add(pool.ttl).Before(pool.clock.Now()) {
			toDeleteKeys = append(toDeleteKeys, key)
			manager.Close(logger)
		}
	}

	for _, key := range toDeleteKeys {
		delete(pool.pool, key)
	}

	logger.Debug("after-collect", lager.Data{"pool-size": len(pool.pool)})

	return nil
}

func NewVarSourcePool(ttl time.Duration, clock clock.Clock) VarSourcePool {
	return &varSourcePool{
		pool:  map[string]*inPoolManager{},
		lock:  sync.Mutex{},
		ttl:   ttl,
		clock: clock,
	}
}
