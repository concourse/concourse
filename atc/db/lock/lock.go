package lock

import (
	"database/sql"
	"errors"
	"fmt"
	"hash/crc32"
	"strconv"
	"strings"
	"sync"

	"code.cloudfoundry.org/lager"
)

const (
	LockTypeResourceConfigChecking = iota
	LockTypeBuildTracking
	LockTypePipelineScheduling
	LockTypeBatch
	LockTypeVolumeCreating
	LockTypeContainerCreating
	LockTypeDatabaseMigration
)

var ErrLostLock = errors.New("lock was lost while held, possibly due to connection breakage")

func NewBuildTrackingLockID(buildID int) LockID {
	return LockID{LockTypeBuildTracking, buildID}
}

func NewResourceConfigCheckingLockID(resourceConfigID int) LockID {
	return LockID{LockTypeResourceConfigChecking, resourceConfigID}
}

func NewPipelineSchedulingLockLockID(pipelineID int) LockID {
	return LockID{LockTypePipelineScheduling, pipelineID}
}

func NewTaskLockID(taskName string) LockID {
	return LockID{LockTypeBatch, lockIDFromString(taskName)}
}

func NewVolumeCreatingLockID(volumeID int) LockID {
	return LockID{LockTypeVolumeCreating, volumeID}
}

func NewContainerCreatingLockID(containerID int) LockID {
	return LockID{LockTypeContainerCreating, containerID}
}

func NewDatabaseMigrationLockID() LockID {
	return LockID{LockTypeDatabaseMigration}
}

//go:generate counterfeiter . LockFactory

type LockFactory interface {
	Acquire(logger lager.Logger, ids LockID) (Lock, bool, error)
}

type lockFactory struct {
	db           LockDB
	locks        lockRepo
	acquireMutex *sync.Mutex
}

func NewLockFactory(conn *sql.DB) LockFactory {
	return &lockFactory{
		db: &lockDB{
			conn:  conn,
			mutex: &sync.Mutex{},
		},
		locks: lockRepo{
			locks: map[string]bool{},
			mutex: &sync.Mutex{},
		},
		acquireMutex: &sync.Mutex{},
	}
}

func NewTestLockFactory(db LockDB) LockFactory {
	return &lockFactory{
		db: db,
		locks: lockRepo{
			locks: map[string]bool{},
			mutex: &sync.Mutex{},
		},
		acquireMutex: &sync.Mutex{},
	}
}

func (f *lockFactory) Acquire(logger lager.Logger, id LockID) (Lock, bool, error) {
	l := &lock{
		logger:       logger,
		db:           f.db,
		id:           id,
		locks:        f.locks,
		acquireMutex: f.acquireMutex,
	}

	acquired, err := l.Acquire()
	if err != nil {
		return nil, false, err
	}

	if !acquired {
		return nil, false, nil
	}

	return l, true, nil
}

//go:generate counterfeiter . Lock

type Lock interface {
	Release() error
}

//go:generate counterfeiter . LockDB

type LockDB interface {
	Acquire(id LockID) (bool, error)
	Release(id LockID) (bool, error)
}

type lock struct {
	id LockID

	logger       lager.Logger
	db           LockDB
	locks        lockRepo
	acquireMutex *sync.Mutex
}

func (l *lock) Acquire() (bool, error) {
	l.acquireMutex.Lock()
	defer l.acquireMutex.Unlock()

	logger := l.logger.Session("acquire", lager.Data{"id": l.id})

	if l.locks.IsRegistered(l.id) {
		logger.Debug("not-acquired-already-held-locally")
		return false, nil
	}

	acquired, err := l.db.Acquire(l.id)
	if err != nil {
		logger.Error("failed-to-register-in-db", err)
		return false, err
	}

	if !acquired {
		logger.Debug("not-acquired-already-held-in-db")
		return false, nil
	}

	l.locks.Register(l.id)

	logger.Debug("acquired")

	return true, nil
}

func (l *lock) Release() error {
	logger := l.logger.Session("release", lager.Data{"id": l.id})

	released, err := l.db.Release(l.id)
	if err != nil {
		logger.Error("failed-to-release-in-db-but-continuing-anyway", err)
	}

	l.locks.Unregister(l.id)

	if !released {
		logger.Error("failed-to-release", ErrLostLock)
		return ErrLostLock
	}

	logger.Debug("released")

	return nil
}

type lockDB struct {
	conn  *sql.DB
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

func (db *lockDB) Release(id LockID) (bool, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	var released bool
	err := db.conn.QueryRow(`SELECT pg_advisory_unlock(`+id.toDBParams()+`)`, id.toDBArgs()...).Scan(&released)
	if err != nil {
		return false, err
	}

	return released, nil
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
