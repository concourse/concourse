package db

import (
	"sync"

	"code.cloudfoundry.org/lager"
	"github.com/jackc/pgx"
)

//go:generate counterfeiter . LockFactory

type LockFactory interface {
	NewLock(logger lager.Logger, lockID int) Lock
}

type lockFactory struct {
	db    lockDB
	locks lockRepo
}

func NewLockFactory(conn *pgx.Conn) LockFactory {
	return &lockFactory{
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

func (f *lockFactory) NewLock(logger lager.Logger, lockID int) Lock {
	return &lock{
		logger: logger,
		db:     f.db,
		id:     lockID,
		locks:  f.locks,
	}
}

//go:generate counterfeiter . Lock

type Lock interface {
	Acquire() (bool, error)
	Release() error
	AfterRelease(func() error)
}

type lock struct {
	logger lager.Logger
	db     lockDB

	id    int
	locks lockRepo

	afterRelease func() error
}

func (l *lock) Acquire() (bool, error) {
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

func (l *lock) Release() error {
	l.db.Release(l.id)

	l.locks.Unregister(l.id)

	if l.afterRelease != nil {
		return l.afterRelease()
	}

	return nil
}

func (l *lock) AfterRelease(afterReleaseFunc func() error) {
	l.afterRelease = afterReleaseFunc
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
