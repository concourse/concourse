package db

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/garyburd/redigo/redis"

	"github.com/winston-ci/winston/builds"
)

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

func (db *redisDB) CreateBuild(job string) (builds.Build, error) {
	conn := db.pool.Get()
	defer conn.Close()

	id, err := redis.Int(conn.Do("INCR", fmt.Sprintf(currentBuildIDKey, job)))
	if err != nil {
		return builds.Build{}, err
	}

	conn.Send("MULTI")

	conn.Send("ZADD", fmt.Sprintf(buildIDsKey, job), -id, id)

	conn.Send(
		"HMSET", fmt.Sprintf(buildKey, job, id),
		"ID", id,
		"Status", builds.StatusPending,
	)

	if _, err := conn.Do("EXEC"); err != nil {
		return builds.Build{}, err
	}

	return builds.Build{
		ID:     id,
		Status: builds.StatusPending,
	}, nil
}

func (db *redisDB) StartBuild(job string, id int, serial bool) (builds.Build, error) {
	conn := db.pool.Get()
	defer conn.Close()

	if serial {
		err := conn.Send("WATCH", fmt.Sprintf(buildIDsKey, job))
		if err != nil {
			return builds.Build{}, err
		}

		currentIDs, err := redis.Values(conn.Do("ZREVRANGE", fmt.Sprintf(buildIDsKey, job), "0", "0"))
		if err != nil {
			return builds.Build{}, err
		}

		var currentID int

		_, err = redis.Scan(currentIDs, &currentID)
		if err != nil {
			return builds.Build{}, err
		}

		queuedIDs, err := redis.Values(conn.Do("ZREVRANGEBYSCORE", fmt.Sprintf(buildIDsKey, job), "0", "-inf"))
		if err != nil {
			return builds.Build{}, err
		}

		var nextQueueID int

		_, err = redis.Scan(queuedIDs, &nextQueueID)
		if err != nil {
			return builds.Build{}, err
		}

		if nextQueueID != id {
			err = conn.Send("UNWATCH")
			if err != nil {
				return builds.Build{}, err
			}

			return builds.Build{
				ID:     id,
				Status: builds.StatusPending,
			}, nil
		}

		if currentID == id {
			err = conn.Send("UNWATCH")
			if err != nil {
				return builds.Build{}, err
			}
		} else {
			vals, err := redis.Values(conn.Do("HGETALL", fmt.Sprintf(buildKey, job, currentID)))
			if err != nil {
				return builds.Build{}, err
			}

			var build builds.Build
			if err := redis.ScanStruct(vals, &build); err != nil {
				return builds.Build{}, err
			}

			switch build.Status {
			case builds.StatusSucceeded, builds.StatusFailed, builds.StatusErrored:
			default:
				err = conn.Send("UNWATCH")
				if err != nil {
					return builds.Build{}, err
				}

				return builds.Build{
					ID:     id,
					Status: builds.StatusPending,
				}, nil
			}
		}
	}

	err := conn.Send("MULTI")
	if err != nil {
		return builds.Build{}, err
	}

	err = conn.Send("ZADD", fmt.Sprintf(buildIDsKey, job), id, id)
	if err != nil {
		return builds.Build{}, err
	}

	err = conn.Send(
		"HMSET", fmt.Sprintf(buildKey, job, id),
		"Status", builds.StatusStarted,
	)
	if err != nil {
		return builds.Build{}, err
	}

	transacted, err := conn.Do("EXEC")
	if err != nil {
		return builds.Build{}, err
	}

	if transacted == nil {
		return builds.Build{
			ID:     id,
			Status: builds.StatusPending,
		}, nil
	}

	return builds.Build{
		ID:     id,
		Status: builds.StatusStarted,
	}, nil
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

func (db *redisDB) SaveBuildLog(job string, build int, log []byte) error {
	conn := db.pool.Get()
	defer conn.Close()

	_, err := conn.Do("SET", fmt.Sprintf(logsKey, job, build), log)
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
