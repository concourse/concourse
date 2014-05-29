package db

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/garyburd/redigo/redis"

	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/config"
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
	buildInputMetadataKey = "build:%s:%d:%s:metadata" // job, build id, resource name => [input metadata]

	logsKey = "logs:%s:%d" // job, build id => build log

	currentSourceKey = "current_source:%s:%s" // job, resource name => source

	outputsKey       = "output:%s:%s"      // job, resource name => [source]
	commonOutputsKey = "common_outputs:%s" // ephemeral; [unique id for jobs + resource] => [set of common outputs]
)

func (db *redisDB) Builds(job string) ([]builds.Build, error) {
	conn := db.pool.Get()
	defer conn.Close()

	ids, err := redis.Values(conn.Do("SMEMBERS", fmt.Sprintf(buildIDsKey, job)))
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

	conn.Send("SADD", fmt.Sprintf(buildIDsKey, job), id)

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

func (db *redisDB) GetCurrentBuild(job string) (builds.Build, error) {
	conn := db.pool.Get()
	defer conn.Close()

	id, err := redis.Int(conn.Do("GET", fmt.Sprintf(currentBuildIDKey, job)))
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

	conn.Send("SET", fmt.Sprintf(buildInputSourceKey, job, id, input.Name), []byte(input.Source))

	conn.Send("RPUSH", append([]interface{}{fmt.Sprintf(buildInputMetadataKey, job, id, input.Name)}, listVals...)...)

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

		input.Source = config.Source(sourceBytes)

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

func (db *redisDB) GetCurrentSource(job, input string) (config.Source, error) {
	conn := db.pool.Get()
	defer conn.Close()

	sourceBytes, err := redis.Bytes(conn.Do("GET", fmt.Sprintf(currentSourceKey, job, input)))
	if err != nil {
		return nil, err
	}

	return config.Source(sourceBytes), nil
}

func (db *redisDB) SaveCurrentSource(job, input string, source config.Source) error {
	conn := db.pool.Get()
	defer conn.Close()

	_, err := conn.Do("SET", fmt.Sprintf(currentSourceKey, job, input), []byte(source))
	return err
}

func (db *redisDB) SaveOutputSource(job string, build int, resourceName string, source config.Source) error {
	conn := db.pool.Get()
	defer conn.Close()

	_, err := conn.Do("ZADD", fmt.Sprintf(outputsKey, job, resourceName), strconv.Itoa(build), []byte(source))
	return err
}

func (db *redisDB) GetCommonOutputs(jobs []string, resourceName string) ([]config.Source, error) {
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

	sourcesBytes, err := redis.Values(conn.Do("ZRANGE", commonKey, "0", "-1"))
	if err != nil {
		return nil, err
	}

	sources := make([]config.Source, len(sourcesBytes))
	for i, iface := range sourcesBytes {
		bytes, err := redis.Bytes(iface, nil)
		if err != nil {
			return nil, err
		}

		sources[i] = config.Source(bytes)
	}

	return sources, nil
}
