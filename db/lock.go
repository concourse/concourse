package db

import (
	"fmt"
	"hash/crc32"
	"strconv"
	"strings"
	"sync"

	"code.cloudfoundry.org/lager"
	"github.com/jackc/pgx"
)

func buildTrackingLockID(buildID int) LockID { return LockID{buildID} }

func resourceCheckingLockID(pipelineID int, resourceName string) LockID {
	return LockID{pipelineID, lockIDFromString(resourceName)}
}

func resourceTypeCheckingLockID(pipelineID int, resourceTypeName string) LockID {
	return LockID{pipelineID, lockIDFromString(resourceTypeName)}
}

func pipelineSchedulingLockLockID(buildID int) LockID { return LockID{buildID} }

func resourceCheckingForJobLockID(pipelineID int, jobName string) LockID {
	return LockID{pipelineID, lockIDFromString(jobName)}
}

func taskLockID(taskName string) LockID {
	return LockID{lockIDFromString(taskName)}
}

//go:generate counterfeiter . LockFactory

type LockFactory interface {
	NewLock(logger lager.Logger, ids LockID) Lock
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
			locks: map[string]bool{},
			mutex: &sync.Mutex{},
		},
	}
}

func (f *lockFactory) NewLock(logger lager.Logger, id LockID) Lock {
	return &lock{
		logger: logger,
		db:     f.db,
		id:     id,
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

	id    LockID
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

func (db *lockDB) Acquire(id LockID) (bool, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	var acquired bool
	err := db.conn.QueryRow(`SELECT pg_try_advisory_lock(`+id.toDBParams()+`)`, id.toDBArgs()...).Scan(&acquired)
	if err != nil {
		return false, err
	}

	return acquired, nil
}

func (db *lockDB) Release(id LockID) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	_, err := db.conn.Exec(`SELECT pg_advisory_unlock(`+id.toDBParams()+`)`, id.toDBArgs()...)
	return err
}

type lockRepo struct {
	locks map[string]bool
	mutex *sync.Mutex
}

func (lr lockRepo) IsRegistered(id LockID) bool {
	lr.mutex.Lock()
	defer lr.mutex.Unlock()

	if _, ok := lr.locks[id.toKey()]; ok {
		return true
	}
	return false
}

func (lr lockRepo) Register(id LockID) {
	lr.mutex.Lock()
	defer lr.mutex.Unlock()

	lr.locks[id.toKey()] = true
}

func (lr lockRepo) Unregister(id LockID) {
	lr.mutex.Lock()
	defer lr.mutex.Unlock()

	delete(lr.locks, id.toKey())
}

type LockID []int

func (l LockID) toKey() string {
	s := []string{}
	for i := range l {
		s = append(s, strconv.Itoa(l[i]))
	}
	return strings.Join(s, "+")
}

func (l LockID) toDBParams() string {
	s := []string{}
	for i := range l {
		s = append(s, fmt.Sprintf("$%d", i+1))
	}

	return strings.Join(s, ",")
}

func (l LockID) toDBArgs() []interface{} {
	result := []interface{}{}
	for i := range l {
		result = append(result, l[i])
	}

	return result
}

func lockIDFromString(taskName string) int {
	return int(int32(crc32.ChecksumIEEE([]byte(taskName))))
}
