package creds

import (
	"sync"
	"time"

	"encoding/json"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . VarSourcePool

type VarSourcePool interface {
	FindOrCreate(lager.Logger, map[string]interface{}, ManagerFactory) (Secrets, error)
	Size() int
	Close()
}

type inPoolManager struct {
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
	return m.secretsFactory.NewSecrets()
}

type varSourcePool struct {
	pool  map[string]*inPoolManager
	lock  sync.Mutex
	ttl   time.Duration
	clock clock.Clock

	closeOnce sync.Once
	closed    chan struct{}
}

func NewVarSourcePool(
	logger lager.Logger,
	ttl time.Duration,
	collectInterval time.Duration,
	clock clock.Clock,
) VarSourcePool {
	pool := &varSourcePool{
		pool:  map[string]*inPoolManager{},
		lock:  sync.Mutex{},
		ttl:   ttl,
		clock: clock,

		closeOnce: sync.Once{},
		closed:    make(chan struct{}),
	}

	go pool.collectLoop(
		logger.Session("collect"),
		collectInterval,
	)

	return pool
}

func (pool *varSourcePool) Size() int {
	pool.lock.Lock()
	defer pool.lock.Unlock()
	return len(pool.pool)
}

func (pool *varSourcePool) FindOrCreate(logger lager.Logger, config map[string]interface{}, factory ManagerFactory) (Secrets, error) {
	b, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	key := string(b)

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
			clock:          pool.clock,
			manager:        manager,
			secretsFactory: secretsFactory,
		}
	} else {
		logger.Debug("found-existing-credential-manager")
	}

	return pool.pool[key].NewSecrets(), nil
}

func (pool *varSourcePool) Close() {
	pool.closeOnce.Do(func() {
		close(pool.closed)
	})
}

func (pool *varSourcePool) collectLoop(logger lager.Logger, interval time.Duration) {
	ticker := pool.clock.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-pool.closed:
			pool.collect(logger.Session("close"), true)
			return
		case <-ticker.C():
			pool.collect(logger.Session("tick"), false)
		}
	}
}

func (pool *varSourcePool) collect(logger lager.Logger, all bool) error {
	pool.lock.Lock()
	defer pool.lock.Unlock()

	logger.Debug("before", lager.Data{"size": len(pool.pool)})

	toDeleteKeys := []string{}
	for key, manager := range pool.pool {
		if all || manager.lastUseTime.Add(pool.ttl).Before(pool.clock.Now()) {
			toDeleteKeys = append(toDeleteKeys, key)
			manager.Close(logger)
		}
	}

	for _, key := range toDeleteKeys {
		delete(pool.pool, key)
	}

	logger.Debug("after", lager.Data{"size": len(pool.pool)})

	return nil
}
