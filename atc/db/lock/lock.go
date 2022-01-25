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
	"github.com/cornelk/hashmap"
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
	CountOfLockTypes // Keep this line as the last item
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

//go:generate counterfeiter . LockFactory

type LockFactory interface {
	Acquire(logger lager.Logger, ids LockID) (Lock, bool, error)
}

type lockFactory struct {
	db    [CountOfLockTypes]LockDB
	locks [CountOfLockTypes]LockRepo
	mutex [CountOfLockTypes]Mutex

	acquireFunc LogFunc
	releaseFunc LogFunc
}

type LogFunc func(logger lager.Logger, id LockID)

func NewLockFactory(
	conn [CountOfLockTypes]*sql.DB,
	acquire LogFunc,
	release LogFunc,
	maxTrackingBuilds int,
	maxCheckingResourceScopes int,
) LockFactory {
	dbs := [CountOfLockTypes]LockDB{}
	locks := [CountOfLockTypes]LockRepo{}
	mutex := [CountOfLockTypes]Mutex{}
	for i := 0; i < CountOfLockTypes; i++ {
		dbs[i] = &lockDB{
			conn:  conn[i],
			mutex: &sync.Mutex{},
		}

		capacity := 0
		switch i {
		case LockTypeResourceConfigChecking:
			capacity = maxCheckingResourceScopes
		case LockTypeBuildTracking:
			capacity = maxTrackingBuilds
		}
		locks[i] = newLockRepo(capacity)

		if capacity == 0 {
			mutex[i] = noopMutex{}
		} else {
			mutex[i] = &sync.Mutex{}
		}
	}

	return &lockFactory{
		db:          dbs,
		acquireFunc: acquire,
		releaseFunc: release,
		locks:       locks,
		mutex:       mutex,
	}
}

func NewTestLockFactory(dbs [CountOfLockTypes]LockDB) LockFactory {
	locks := [CountOfLockTypes]LockRepo{}
	for i := 0; i < CountOfLockTypes; i++ {
		locks[i] = newLockRepo(0)
	}

	return &lockFactory{
		db:          dbs,
		locks:       locks,
		acquireFunc: func(logger lager.Logger, id LockID) {},
		releaseFunc: func(logger lager.Logger, id LockID) {},
	}
}

func (f *lockFactory) Acquire(logger lager.Logger, id LockID) (Lock, bool, error) {
	lockType := id[0]
	l := &lock{
		logger:       logger,
		db:           f.db[lockType],
		id:           id,
		locks:        f.locks[lockType],
		acquireMutex: f.mutex[lockType],
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
	locks        LockRepo
	acquireMutex Mutex

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

	if l.locks.IsFull() {
		return false, fmt.Errorf("lock %d capacity full", l.id[0])
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

type LockRepo interface {
	IsRegistered(LockID) bool
	Register(LockID)
	Unregister(LockID)
	IsFull() bool
}

type lockRepo struct {
	locks    hashmap.HashMap
	capacity int
}

func newLockRepo(capacity int) *lockRepo {
	return &lockRepo{
		locks:    hashmap.HashMap{},
		capacity: capacity,
	}
}

func (lr *lockRepo) IsFull() bool {
	if lr.capacity == 0 {
		return false
	}
	return lr.locks.Len() >= lr.capacity
}

func (lr *lockRepo) IsRegistered(id LockID) bool {
	if _, ok := lr.locks.Get(id.toKey()); ok {
		return true
	}
	return false
}

func (lr *lockRepo) Register(id LockID) {
	lr.locks.Set(id.toKey(), true)
}

func (lr *lockRepo) Unregister(id LockID) {
	lr.locks.Del(id.toKey())
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

type Mutex interface {
	Lock()
	Unlock()
}

type noopMutex struct{}

func (m noopMutex) Lock() {
}

func (m noopMutex) Unlock() {
}
