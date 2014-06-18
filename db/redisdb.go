package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/garyburd/redigo/redis"

	"github.com/winston-ci/winston/builds"
)

var ErrBuildCreateConflict = errors.New("error creating build atomically")

type redisDB struct {
	pool *redis.Pool
}

func NewRedis(pool *redis.Pool) DB {
	return &redisDB{
		pool: pool,
	}
}

const (
	buildIDsKey           = "build_ids:%s"            // job => [build id]
	currentBuildIDKey     = "current_build_id:%s"     // job => build id
	buildKey              = "build:%s:%d"             // job, build id => build
	buildInputsKey        = "build:%s:%d:inputs"      // job, build id => [resource name]
	buildInputSourceKey   = "build:%s:%d:%s:source"   // job, build id, resource name => source
	buildInputVersionKey  = "build:%s:%d:%s:version"  // job, build id, resource name => version
	buildInputMetadataKey = "build:%s:%d:%s:metadata" // job, build id, resource name => [input metadata]

	logsKey = "logs:%s:%d" // job, build id => build log

	currentVersionKey = "current_version:%s:%s" // job, resource name => version

	outputsKey       = "output:%s:%s"      // job, resource name => [version]
	commonOutputsKey = "common_outputs:%s" // ephemeral; [unique id for jobs + resource] => [set of common outputs]
)

func (db *redisDB) Builds(job string) ([]builds.Build, error) {
	conn := db.pool.Get()
	defer conn.Close()

	ids, err := redis.Values(conn.Do("ZREVRANGE", fmt.Sprintf(buildIDsKey, job), "0", "-1"))
	if err != nil {
		return nil, err
	}

	bs := make([]builds.Build, len(ids))
	for i := 0; len(ids) > 0; i++ {
		var id int
		ids, err = redis.Scan(ids, &id)
		if err != nil {
			return nil, err
		}

		build, err := db.GetBuild(job, id)
		if err != nil {
			return nil, err
		}

		bs[i] = build
	}

	return bs, nil
}

var ErrInputNotDetermined = errors.New("input not yet determined; cannot know if redundant")
var ErrInputRedundant = errors.New("resource version already used for input")

var ErrOutputNotDetermined = errors.New("output not yet determined; cannot know if redundant")
var ErrOutputRedundant = errors.New("resource version came from output")

func (db *redisDB) AttemptBuild(job string, input string, version builds.Version, serial bool) (builds.Build, error) {
	versionJSON, err := json.Marshal(version)
	if err != nil {
		return builds.Build{}, err
	}

	conn := db.pool.Get()
	defer conn.Close()

	err = conn.Send("WATCH", fmt.Sprintf(currentBuildIDKey, job))
	if err != nil {
		return builds.Build{}, err
	}

	defer conn.Send("UNWATCH")

	currentID, err := db.currentBuildID(conn, job)
	if err == nil {
		activeInputVersion, err := redis.Bytes(conn.Do("GET", fmt.Sprintf(buildInputVersionKey, job, currentID, input)))
		if err != nil {
			return builds.Build{}, ErrInputNotDetermined
		}

		if reflect.DeepEqual(activeInputVersion, versionJSON) {
			return builds.Build{}, ErrInputRedundant
		}

		activeOutputVersions, err := redis.Values(conn.Do("ZRANGEBYSCORE", fmt.Sprintf(outputsKey, job, input), strconv.Itoa(currentID), strconv.Itoa(currentID)))
		if err != nil {
			if serial {
				return builds.Build{}, ErrOutputNotDetermined
			}
		} else {
			var outputVersion []byte
			_, err = redis.Scan(activeOutputVersions, &outputVersion)
			if err != nil {
				if serial {
					return builds.Build{}, ErrOutputNotDetermined
				}
			} else {
				if reflect.DeepEqual(outputVersion, versionJSON) {
					return builds.Build{}, ErrOutputRedundant
				}
			}
		}
	}

	id := currentID + 1

	err = conn.Send("MULTI")
	if err != nil {
		return builds.Build{}, err
	}

	err = conn.Send("SET", fmt.Sprintf(currentBuildIDKey, job), id)
	if err != nil {
		return builds.Build{}, err
	}

	build, err := db.createBuild(conn, job, id, input, versionJSON)
	if err != nil {
		return builds.Build{}, err
	}

	return build, nil
}

func (db *redisDB) CreateBuild(job string) (builds.Build, error) {
	conn := db.pool.Get()
	defer conn.Close()

	id, err := redis.Int(conn.Do("INCR", fmt.Sprintf(currentBuildIDKey, job)))
	if err != nil {
		return builds.Build{}, err
	}

	err = conn.Send("MULTI")
	if err != nil {
		return builds.Build{}, err
	}

	return db.createBuild(conn, job, id, "", nil)
}

func (db *redisDB) createBuild(conn redis.Conn, job string, id int, input string, versionJSON []byte) (builds.Build, error) {
	err := conn.Send("ZADD", fmt.Sprintf(buildIDsKey, job), -id, id)
	if err != nil {
		return builds.Build{}, err
	}

	if versionJSON != nil {
		err = conn.Send("SET", fmt.Sprintf(buildInputVersionKey, job, id, input), versionJSON)
		if err != nil {
			return builds.Build{}, err
		}
	}

	err = conn.Send(
		"HMSET", fmt.Sprintf(buildKey, job, id),
		"ID", id,
		"Status", builds.StatusPending,
	)
	if err != nil {
		return builds.Build{}, err
	}

	transacted, err := conn.Do("EXEC")
	if err != nil {
		return builds.Build{}, err
	}

	if transacted == nil {
		return builds.Build{}, ErrBuildCreateConflict
	}

	return builds.Build{
		ID:     id,
		Status: builds.StatusPending,
	}, nil
}

func (db *redisDB) ScheduleBuild(job string, id int, serial bool) (bool, error) {
	conn := db.pool.Get()
	defer conn.Close()

	// watch for state changes (i.e. build aborted)
	err := conn.Send("WATCH", fmt.Sprintf(buildKey, job, id))
	if err != nil {
		return false, err
	}

	defer conn.Send("UNWATCH")

	if serial {
		nextPendingID, err := db.nextPendingBuildID(conn, job)
		if err != nil {
			return false, err
		}

		// only schedule if this build is the next one queued
		if nextPendingID != id {
			return false, nil
		}

		activeID, err := db.currentActiveBuildID(conn, job)
		if err != nil {
			return false, err
		}

		// only schedule if the current build is not running
		if activeID != id {
			vals, err := redis.Values(conn.Do("HGETALL", fmt.Sprintf(buildKey, job, activeID)))
			if err != nil {
				return false, err
			}

			var build builds.Build
			if err := redis.ScanStruct(vals, &build); err != nil {
				return false, err
			}

			switch build.Status {
			case builds.StatusPending, builds.StatusStarted:
				return false, nil
			default:
			}
		}
	}

	build, err := db.GetBuild(job, id)
	if err != nil {
		return false, err
	}

	if build.Status != builds.StatusPending {
		return false, nil
	}

	err = conn.Send("MULTI")
	if err != nil {
		return false, err
	}

	err = conn.Send("ZADD", fmt.Sprintf(buildIDsKey, job), id, id)
	if err != nil {
		return false, err
	}

	transacted, err := conn.Do("EXEC")
	if err != nil {
		return false, err
	}

	if transacted == nil {
		return false, nil
	}

	return true, nil
}

func (db *redisDB) StartBuild(job string, id int, abortURL string) (bool, error) {
	conn := db.pool.Get()
	defer conn.Close()

	err := conn.Send("WATCH", fmt.Sprintf(buildKey, job, id))
	if err != nil {
		return false, err
	}

	defer conn.Send("UNWATCH")

	build, err := db.GetBuild(job, id)
	if err != nil {
		return false, err
	}

	if build.Status != builds.StatusPending {
		return false, nil
	}

	err = conn.Send("MULTI")
	if err != nil {
		return false, err
	}

	err = conn.Send(
		"HMSET", fmt.Sprintf(buildKey, job, id),
		"Status", builds.StatusStarted,
		"AbortURL", abortURL,
	)
	if err != nil {
		return false, err
	}

	transacted, err := conn.Do("EXEC")
	if err != nil {
		return false, err
	}

	if transacted == nil {
		return false, nil
	}

	return true, nil
}

func (db *redisDB) AbortBuild(job string, id int) error {
	conn := db.pool.Get()
	defer conn.Close()

	err := conn.Send("MULTI")
	if err != nil {
		return err
	}

	err = conn.Send("ZADD", fmt.Sprintf(buildIDsKey, job), id, id)
	if err != nil {
		return err
	}

	err = conn.Send(
		"HMSET", fmt.Sprintf(buildKey, job, id),
		"Status", builds.StatusAborted,
	)

	_, err = conn.Do("EXEC")
	if err != nil {
		return err
	}

	return err
}

func (db *redisDB) GetCurrentBuild(job string) (builds.Build, error) {
	conn := db.pool.Get()
	defer conn.Close()

	ids, err := redis.Values(conn.Do("ZREVRANGE", fmt.Sprintf(buildIDsKey, job), "0", "0"))
	if err != nil {
		return builds.Build{}, err
	}

	var id int

	_, err = redis.Scan(ids, &id)
	if err != nil {
		return builds.Build{}, err
	}

	return db.GetBuild(job, id)
}

func (db *redisDB) SaveBuildInput(job string, id int, input builds.Input) error {
	conn := db.pool.Get()
	defer conn.Close()

	listVals := []interface{}{}
	for _, field := range input.Metadata {
		listVals = append(listVals, field.Name, field.Value)
	}

	conn.Send("MULTI")

	conn.Send("RPUSH", fmt.Sprintf(buildInputsKey, job, id), input.Name)

	sourceJSON, err := json.Marshal(input.Source)
	if err != nil {
		return err
	}

	conn.Send("SET", fmt.Sprintf(buildInputSourceKey, job, id, input.Name), sourceJSON)

	versionJSON, err := json.Marshal(input.Version)
	if err != nil {
		return err
	}

	conn.Send("SET", fmt.Sprintf(buildInputVersionKey, job, id, input.Name), versionJSON)

	if len(listVals) > 0 {
		conn.Send("RPUSH", append([]interface{}{fmt.Sprintf(buildInputMetadataKey, job, id, input.Name)}, listVals...)...)
	}

	if _, err := conn.Do("EXEC"); err != nil {
		return err
	}

	return nil
}

func (db *redisDB) SaveBuildStatus(job string, id int, status builds.Status) error {
	conn := db.pool.Get()
	defer conn.Close()

	err := conn.Send("HSET", fmt.Sprintf(buildKey, job, id), "Status", status)
	if err != nil {
		return err
	}

	return nil
}

func (db *redisDB) GetBuild(job string, id int) (builds.Build, error) {
	conn := db.pool.Get()
	defer conn.Close()

	vals, err := redis.Values(conn.Do("HGETALL", fmt.Sprintf(buildKey, job, id)))
	if err != nil {
		return builds.Build{}, err
	}

	var build builds.Build
	if err := redis.ScanStruct(vals, &build); err != nil {
		return builds.Build{}, err
	}

	inputNames, err := redis.Strings(conn.Do("LRANGE", fmt.Sprintf(buildInputsKey, job, id), "0", "-1"))
	if err != nil {
		return builds.Build{}, err
	}

	for _, name := range inputNames {
		input := builds.Input{
			Name: name,
		}

		sourceBytes, err := redis.Bytes(conn.Do("GET", fmt.Sprintf(buildInputSourceKey, job, id, name)))
		if err != nil {
			return builds.Build{}, err
		}

		err = json.Unmarshal(sourceBytes, &input.Source)
		if err != nil {
			return builds.Build{}, err
		}

		versionBytes, err := redis.Bytes(conn.Do("GET", fmt.Sprintf(buildInputVersionKey, job, id, name)))
		if err != nil {
			return builds.Build{}, err
		}

		err = json.Unmarshal(versionBytes, &input.Version)
		if err != nil {
			return builds.Build{}, err
		}

		metadataFields, err := redis.Values(conn.Do("LRANGE", fmt.Sprintf(buildInputMetadataKey, job, id, name), "0", "-1"))
		if err != nil {
			return builds.Build{}, err
		}

		for len(metadataFields) > 0 {
			var name, value string
			metadataFields, err = redis.Scan(metadataFields, &name, &value)
			if err != nil {
				return builds.Build{}, err
			}

			input.Metadata = append(input.Metadata, builds.MetadataField{
				Name:  name,
				Value: value,
			})
		}

		build.Inputs = append(build.Inputs, input)
	}

	return build, nil
}

func (db *redisDB) BuildLog(job string, build int) ([]byte, error) {
	conn := db.pool.Get()
	defer conn.Close()

	return redis.Bytes(conn.Do("GET", fmt.Sprintf(logsKey, job, build)))
}

func (db *redisDB) AppendBuildLog(job string, build int, log []byte) error {
	conn := db.pool.Get()
	defer conn.Close()

	_, err := conn.Do("APPEND", fmt.Sprintf(logsKey, job, build), log)
	return err
}

func (db *redisDB) GetCurrentVersion(job, input string) (builds.Version, error) {
	conn := db.pool.Get()
	defer conn.Close()

	versionBytes, err := redis.Bytes(conn.Do("GET", fmt.Sprintf(currentVersionKey, job, input)))
	if err != nil {
		return nil, err
	}

	var version builds.Version

	err = json.Unmarshal(versionBytes, &version)
	return version, err
}

func (db *redisDB) SaveCurrentVersion(job, input string, version builds.Version) error {
	conn := db.pool.Get()
	defer conn.Close()

	versionBytes, err := json.Marshal(version)
	if err != nil {
		return err
	}

	_, err = conn.Do("SET", fmt.Sprintf(currentVersionKey, job, input), versionBytes)
	return err
}

func (db *redisDB) SaveOutputVersion(job string, build int, resourceName string, version builds.Version) error {
	conn := db.pool.Get()
	defer conn.Close()

	versionBytes, err := json.Marshal(version)
	if err != nil {
		return err
	}

	_, err = conn.Do("ZADD", fmt.Sprintf(outputsKey, job, resourceName), strconv.Itoa(build), versionBytes)
	return err
}

func (db *redisDB) GetCommonOutputs(jobs []string, resourceName string) ([]builds.Version, error) {
	conn := db.pool.Get()
	defer conn.Close()

	commonKey := fmt.Sprintf(commonOutputsKey, strings.Join(append(jobs, resourceName), ":"))

	outputKeys := []interface{}{}
	for _, job := range jobs {
		outputKeys = append(outputKeys, fmt.Sprintf(outputsKey, job, resourceName))
	}

	_, err := conn.Do(
		"ZINTERSTORE",
		append(
			[]interface{}{commonKey, strconv.Itoa(len(jobs))},
			outputKeys...,
		)...,
	)

	versionsBytes, err := redis.Values(conn.Do("ZRANGE", commonKey, "0", "-1"))
	if err != nil {
		return nil, err
	}

	versions := make([]builds.Version, len(versionsBytes))
	for i, iface := range versionsBytes {
		bytes, err := redis.Bytes(iface, nil)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(bytes, &versions[i])
		if err != nil {
			return nil, err
		}
	}

	return versions, nil
}

func (db *redisDB) currentBuildID(conn redis.Conn, job string) (int, error) {
	return redis.Int(conn.Do("GET", fmt.Sprintf(currentBuildIDKey, job)))
}

func (db *redisDB) currentActiveBuildID(conn redis.Conn, job string) (int, error) {
	currentIDs, err := redis.Values(conn.Do("ZREVRANGE", fmt.Sprintf(buildIDsKey, job), "0", "0"))

	var currentID int
	_, err = redis.Scan(currentIDs, &currentID)
	if err != nil {
		return 0, err
	}

	return currentID, nil
}

func (db *redisDB) nextPendingBuildID(conn redis.Conn, job string) (int, error) {
	pendingdIDs, err := redis.Values(conn.Do("ZREVRANGEBYSCORE", fmt.Sprintf(buildIDsKey, job), "0", "-inf", "LIMIT", "0", "1"))
	if err != nil {
		return 0, err
	}

	var nextPendingID int
	_, err = redis.Scan(pendingdIDs, &nextPendingID)
	if err != nil {
		return 0, err
	}

	return nextPendingID, nil
}
