package db

import (
	"sync"

	"code.cloudfoundry.org/lager"
	"github.com/jackc/pgx"
)

//go:generate counterfeiter . LeaseFactory

type LeaseFactory interface {
	NewLease(logger lager.Logger, lockID int) Lease
}

type leaseFactory struct {
	db    lockDB
	locks lockRepo
}

func NewLeaseFactory(conn *pgx.Conn) LeaseFactory {
	return &leaseFactory{
		db: lockDB{
			conn:  conn,
			mutex: &sync.Mutex{},
		},
		locks: lockRepo{
			locks: map[int]bool{},
			mutex: &sync.Mutex{},
		},
	}
}

func (f *leaseFactory) NewLease(logger lager.Logger, lockID int) Lease {
	return &lease{
		logger: logger,
		db:     f.db,
		id:     lockID,
		locks:  f.locks,
	}
}

//go:generate counterfeiter . Lease

type Lease interface {
	AttemptSign() (bool, error)
	Break() error
	AfterBreak(func() error)
}

type lease struct {
	logger lager.Logger
	db     lockDB

	id    int
	locks lockRepo

	afterBreak func() error
}

func (l *lease) AttemptSign() (bool, error) {
	if l.locks.IsRegistered(l.id) {
		return false, nil
	}

	acquired, err := l.db.Acquire(l.id)
	if err != nil {
		return false, err
	}

	l.locks.Register(l.id)

	return acquired, nil
}

func (l *lease) Break() error {
	l.db.Release(l.id)

	l.locks.Unregister(l.id)

	if l.afterBreak != nil {
		return l.afterBreak()
	}

	return nil
}

func (l *lease) AfterBreak(afterBreakFunc func() error) {
	l.afterBreak = afterBreakFunc
}

type lockDB struct {
	conn  *pgx.Conn
	mutex *sync.Mutex
}

func (db *lockDB) Acquire(name int) (bool, error) {
	var acquired bool
	err := db.conn.QueryRow(`SELECT pg_try_advisory_lock($1)`, name).Scan(&acquired)
	if err != nil {
		return false, err
	}

	return acquired, nil
}

func (db *lockDB) Release(name int) error {
	_, err := db.conn.Exec(`SELECT pg_advisory_unlock($1)`, name)
	return err
}

type lockRepo struct {
	locks map[int]bool
	mutex *sync.Mutex
}

func (lr lockRepo) IsRegistered(lockID int) bool {
	lr.mutex.Lock()
	defer lr.mutex.Unlock()

	if _, ok := lr.locks[lockID]; ok {
		return true
	}
	return false
}

func (lr lockRepo) Register(lockID int) {
	lr.mutex.Lock()
	defer lr.mutex.Unlock()

	lr.locks[lockID] = true
}

func (lr lockRepo) Unregister(lockID int) {
	lr.mutex.Lock()
	defer lr.mutex.Unlock()

	delete(lr.locks, lockID)
}
