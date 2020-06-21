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
	LockTypeBatch
	LockTypeVolumeCreating
	LockTypeContainerCreating
	LockTypeDatabaseMigration
	LockTypeActiveTasks
	LockTypeResourceScanning
	LockTypeJobScheduling
	LockTypeCreateWatchTriggers
)

var ErrLostLock = errors.New("lock was lost while held, possibly due to connection breakage")

func NewBuildTrackingLockID(buildID int) LockID {
	return LockID{LockTypeBuildTracking, buildID}
}

func NewResourceConfigCheckingLockID(resourceConfigID int) LockID {
	return LockID{LockTypeResourceConfigChecking, resourceConfigID}
}

func NewTaskLockID(taskName string) LockID {
	return LockID{LockTypeBatch, lockIDFromString(taskName)}
}

func NewVolumeCreatingLockID(volumeID int) LockID {
	return LockID{LockTypeVolumeCreating, volumeID}
}

func NewDatabaseMigrationLockID() LockID {
	return LockID{LockTypeDatabaseMigration}
}

func NewActiveTasksLockID() LockID {
	return LockID{LockTypeActiveTasks}
}

func NewResourceScanningLockID() LockID {
	return LockID{LockTypeResourceScanning}
}

func NewJobSchedulingLockID(jobID int) LockID {
	return LockID{LockTypeJobScheduling, jobID}
}

func NewCreateWatchTriggersLockID() LockID {
	return LockID{LockTypeCreateWatchTriggers}
}

//go:generate counterfeiter . LockFactory

type LockFactory interface {
	Acquire(logger lager.Logger, ids LockID) (Lock, bool, error)
}

type lockFactory struct {
	db           LockDB
	locks        lockRepo
	acquireMutex *sync.Mutex

	acquireFunc LogFunc
	releaseFunc LogFunc
}

type LogFunc func(logger lager.Logger, id LockID)

func NewLockFactory(
	conn *sql.DB,
	acquire LogFunc,
	release LogFunc,
) LockFactory {
	return &lockFactory{
		db: &lockDB{
			conn:  conn,
			mutex: &sync.Mutex{},
		},
		acquireFunc: acquire,
		releaseFunc: release,
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
		acquireFunc:  func(logger lager.Logger, id LockID) {},
		releaseFunc:  func(logger lager.Logger, id LockID) {},
	}
}

func (f *lockFactory) Acquire(logger lager.Logger, id LockID) (Lock, bool, error) {
	l := &lock{
		logger:       logger,
		db:           f.db,
		id:           id,
		locks:        f.locks,
		acquireMutex: f.acquireMutex,
		acquired:     f.acquireFunc,
		released:     f.releaseFunc,
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

	acquired LogFunc
	released LogFunc
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

	l.acquired(logger, l.id)

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

	l.released(logger, l.id)

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
