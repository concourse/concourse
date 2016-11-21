package lock

import (
	"fmt"
	"hash/crc32"
	"strconv"
	"strings"
	"sync"

	"code.cloudfoundry.org/lager"
)

const (
	LockTypeResourceChecking = iota
	LockTypeResourceTypeChecking
	LockTypeBuildTracking
	LockTypePipelineScheduling
	LockTypeResourceCheckingForJob
	LockTypeBatch
	LockTypeVolumeCreating
)

func NewBuildTrackingLockID(buildID int) LockID {
	return LockID{LockTypeBuildTracking, buildID}
}

func NewResourceCheckingLockID(resourceID int) LockID {
	return LockID{LockTypeResourceChecking, resourceID}
}

func NewResourceTypeCheckingLockID(resourceTypeID int) LockID {
	return LockID{LockTypeResourceTypeChecking, resourceTypeID}
}

func NewPipelineSchedulingLockLockID(pipelineID int) LockID {
	return LockID{LockTypePipelineScheduling, pipelineID}
}

func NewResourceCheckingForJobLockID(jobID int) LockID {
	return LockID{LockTypeResourceCheckingForJob, jobID}
}

func NewTaskLockID(taskName string) LockID {
	return LockID{LockTypeBatch, lockIDFromString(taskName)}
}

func NewVolumeCreatingLockID(volumeID int) LockID {
	return LockID{LockTypeVolumeCreating, volumeID}
}

//go:generate counterfeiter . LockFactory

type LockFactory interface {
	NewLock(logger lager.Logger, ids LockID) Lock
}

type lockFactory struct {
	db           LockDB
	locks        lockRepo
	acquireMutex *sync.Mutex
}

func NewLockFactory(conn *RetryableConn) LockFactory {
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

func (f *lockFactory) NewLock(logger lager.Logger, id LockID) Lock {
	return &lock{
		logger:       logger,
		db:           f.db,
		id:           id,
		locks:        f.locks,
		acquireMutex: f.acquireMutex,
	}
}

//go:generate counterfeiter . Lock

type Lock interface {
	Acquire() (bool, error)
	Release() error
	AfterRelease(func() error)
}

//go:generate counterfeiter . LockDB

type LockDB interface {
	Acquire(id LockID) (bool, error)
	Release(id LockID) error
}

type lock struct {
	id LockID

	logger       lager.Logger
	db           LockDB
	locks        lockRepo
	acquireMutex *sync.Mutex

	afterRelease func() error
}

func (l *lock) Acquire() (bool, error) {
	l.acquireMutex.Lock()
	defer l.acquireMutex.Unlock()

	logger := l.logger.Session("acquire", lager.Data{"id": l.id})

	if l.locks.IsRegistered(l.id) {
		logger.Debug("already-registered")
		return false, nil
	}

	acquired, err := l.db.Acquire(l.id)
	if err != nil {
		logger.Debug("failed-to-register-in-db")
		return false, err
	}

	if !acquired {
		return false, err
	}

	logger.Debug("registering-in-repo")
	l.locks.Register(l.id)

	return true, nil
}

func (l *lock) Release() error {
	logger := l.logger.Session("release", lager.Data{"id": l.id})

	logger.Debug("releasing-in-db")
	l.db.Release(l.id)

	logger.Debug("unregistering-from-repo")
	l.locks.Unregister(l.id)

	if l.afterRelease != nil {
		logger.Debug("running-after-release")
		return l.afterRelease()
	}

	return nil
}

func (l *lock) AfterRelease(afterReleaseFunc func() error) {
	l.afterRelease = afterReleaseFunc
}

type lockDB struct {
	conn  *RetryableConn
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
