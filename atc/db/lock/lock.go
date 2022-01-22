package lock

import (
	"database/sql"
	"errors"
	"fmt"
	"hash/crc32"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/cornelk/hashmap"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

const (
	LockTypeResourceConfigChecking = iota
	LockTypeBuildTracking
	LockTypeBatch
	LockTypeVolumeCreating
	LockTypeContainerCreating
	LockTypeDatabaseMigration
	LockTypeResourceScanning // no longer used, but don't delete it, because we don't want to change lock id
	LockTypeJobScheduling
	LockTypeInMemoryCheckBuildTracking
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

func NewResourceScanningLockID() LockID {
	return LockID{LockTypeResourceScanning}
}

func NewJobSchedulingLockID(jobID int) LockID {
	return LockID{LockTypeJobScheduling, jobID}
}

func NewInMemoryCheckBuildTrackingLockID(checkableType string, checkableId int) LockID {
	return LockID{LockTypeInMemoryCheckBuildTracking, lockIDFromString(fmt.Sprintf("%s-%d", checkableType, checkableId))}
}

//counterfeiter:generate . LockFactory
type LockFactory interface {
	Acquire(logger lager.Logger, ids LockID) (Lock, bool, error)
}

type lockFactory struct {
	db           [CountOfLockTypes]LockDB
	locks        [CountOfLockTypes]LockRepo
	//acquireMutex *sync.Mutex

	acquireFunc LogFunc
	releaseFunc LogFunc
}

type LogFunc func(logger lager.Logger, id LockID)

func NewLockFactory(
	conn [CountOfLockTypes]*sql.DB,
	acquire LogFunc,
	release LogFunc,
) LockFactory {
	dbs := [CountOfLockTypes]LockDB{}
	locks := [CountOfLockTypes]LockRepo{}
	for i := 0; i < CountOfLockTypes; i ++ {
		dbs[i] = &lockDB{
			conn:  conn[i],
			//mutex: &sync.Mutex{},
		}
		locks[i] = &lockRepo{
			//locks: map[string]bool{},
			//mutex: &sync.Mutex{},
			locks: hashmap.HashMap{},
		}
	}

	locks[LockTypeResourceConfigChecking].(*lockRepo).capacity = 100
	locks[LockTypeBuildTracking].(*lockRepo).capacity = 100
	locks[LockTypeInMemoryCheckBuildTracking].(*lockRepo).capacity = 500

	return &lockFactory{
		db: dbs,
		acquireFunc: acquire,
		releaseFunc: release,
		locks: locks,
		//acquireMutex: &sync.Mutex{},
	}
}

//func NewTestLockFactory(db LockDB) LockFactory {
//	return &lockFactory{
//		db: db,
//		locks: lockRepo{
//			//locks: map[string]bool{},
//			//mutex: &sync.Mutex{},
//			locks: hashmap.HashMap{},
//		},
//		acquireMutex: &sync.Mutex{},
//		acquireFunc:  func(logger lager.Logger, id LockID) {},
//		releaseFunc:  func(logger lager.Logger, id LockID) {},
//	}
//}

func (f *lockFactory) Acquire(logger lager.Logger, id LockID) (Lock, bool, error) {
	lockType := id[0]
	l := &lock{
		logger:       logger,
		db:           f.db[lockType],
		id:           id,
		locks:        f.locks[lockType],
		//acquireMutex: f.acquireMutex,
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

//counterfeiter:generate . Lock
type Lock interface {
	Release() error
}

// NoopLock is a fake lock for use when a lock is conditionally acquired.
type NoopLock struct{}

// Release does nothing. Successfully.
func (NoopLock) Release() error { return nil }

//counterfeiter:generate . LockDB
type LockDB interface {
	Acquire(logger lager.Logger, id LockID) (bool, error)
	Release(logger lager.Logger, id LockID) (bool, error)
}

type lock struct {
	id LockID

	logger       lager.Logger
	db           LockDB
	locks        LockRepo
	//acquireMutex *sync.Mutex

	acquired LogFunc
	released LogFunc
}

func (l *lock) Acquire() (bool, error) {
	logger := l.logger.Session("acquire", lager.Data{"id": l.id})

	//l.acquireMutex.Lock()
	//defer l.acquireMutex.Unlock()

	if l.locks.IsFull() {
		return false, fmt.Errorf("lock %d capacity full", l.id[0])
	}

	if l.locks.IsRegistered(l.id) {
		logger.Debug("not-acquired-already-held-locally")
		return false, nil
	}

	acquired, err := l.db.Acquire(logger, l.id)
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

	released, err := l.db.Release(logger, l.id)
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
	//mutex *sync.Mutex
}

func (db *lockDB) Acquire(logger lager.Logger, id LockID) (bool, error) {
	//db.mutex.Lock()
	//defer db.mutex.Unlock()

	start := time.Now()

	var acquired bool
	err := db.conn.QueryRow(`SELECT pg_try_advisory_lock(`+id.toDBParams()+`)`, id.toDBArgs()...).Scan(&acquired)
	if err != nil {
		return false, err
	}

	if time.Now().Sub(start) > time.Second {
		logger.Info("EVAN:db lock too long", lager.Data{"lock-id": id, "duration": (time.Now().Sub(start).Seconds())})
	}

	return acquired, nil
}

func (db *lockDB) Release(logger lager.Logger, id LockID) (bool, error) {
	//db.mutex.Lock()
	//defer db.mutex.Unlock()

	start := time.Now()

	var released bool
	err := db.conn.QueryRow(`SELECT pg_advisory_unlock(`+id.toDBParams()+`)`, id.toDBArgs()...).Scan(&released)
	if err != nil {
		return false, err
	}

	if time.Now().Sub(start) > time.Second {
		logger.Info("EVAN:db unlock too long", lager.Data{"lock-id": id, "duration": (time.Now().Sub(start).Seconds())})
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
	//locks map[string]bool
	//mutex *sync.Mutex
	locks hashmap.HashMap
	capacity int
}

func (lr *lockRepo) IsFull() bool {
	if lr.capacity == 0 {
		return false
	}
	return lr.locks.Len() >= lr.capacity
}

func (lr *lockRepo) IsRegistered(id LockID) bool {
	//lr.mutex.Lock()
	//defer lr.mutex.Unlock()
	//
	//if _, ok := lr.locks[id.toKey()]; ok {
	//	return true
	//}
	if _, ok := lr.locks.Get(id.toKey()); ok {
		return true
	}
	return false
}

func (lr *lockRepo) Register(id LockID) {
	//lr.mutex.Lock()
	//defer lr.mutex.Unlock()
	//
	//lr.locks[id.toKey()] = true
	lr.locks.Set(id.toKey(), true)
}

func (lr *lockRepo) Unregister(id LockID) {
	//lr.mutex.Lock()
	//defer lr.mutex.Unlock()
	//
	//delete(lr.locks, id.toKey())
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
