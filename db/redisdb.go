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

func (db *redisDB) Builds(job string) ([]builds.Build, error) {
	conn := db.pool.Get()
	defer conn.Close()

	ids, err := redis.Strings(conn.Do("SMEMBERS", fmt.Sprintf("build_ids:%s", job)))
	if err != nil {
		return nil, err
	}

	bs := make([]builds.Build, len(ids))
	for i, idStr := range ids {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			panic("invalid build id: " + idStr + " (" + err.Error() + ")")
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

	id, err := redis.Int(conn.Do("INCR", "next_build_id:"+job))
	if err != nil {
		return builds.Build{}, err
	}

	idStr := fmt.Sprintf("%d", id)

	conn.Send("MULTI")

	conn.Send("SADD", "build_ids:"+job, idStr)

	conn.Send(
		"HMSET", "build:"+job+":"+idStr,
		"ID", idStr,
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

func (db *redisDB) SaveBuildStatus(job string, id int, status builds.Status) (builds.Build, error) {
	conn := db.pool.Get()
	defer conn.Close()

	idStr := fmt.Sprintf("%d", id)

	err := conn.Send("MULTI")
	if err != nil {
		return builds.Build{}, err
	}

	err = conn.Send("HSET", "build:"+job+":"+idStr, "Status", status)
	if err != nil {
		return builds.Build{}, err
	}

	err = conn.Send("HGETALL", "build:"+job+":"+idStr)
	if err != nil {
		return builds.Build{}, err
	}

	vals, err := redis.Values(conn.Do("EXEC"))
	vals, err = redis.Values(vals[1], err)
	if err != nil {
		return builds.Build{}, err
	}

	var build builds.Build
	if err := redis.ScanStruct(vals, &build); err != nil {
		return builds.Build{}, err
	}

	return build, nil
}

func (db *redisDB) GetBuild(job string, id int) (builds.Build, error) {
	conn := db.pool.Get()
	defer conn.Close()

	idStr := fmt.Sprintf("%d", id)

	vals, err := redis.Values(conn.Do("HGETALL", "build:"+job+":"+idStr))
	if err != nil {
		return builds.Build{}, err
	}

	var build builds.Build
	if err := redis.ScanStruct(vals, &build); err != nil {
		return builds.Build{}, err
	}

	return build, nil
}

func (db *redisDB) BuildLog(job string, build int) ([]byte, error) {
	conn := db.pool.Get()
	defer conn.Close()

	idStr := fmt.Sprintf("%d", build)

	return redis.Bytes(conn.Do("GET", "logs:"+job+":"+idStr))
}

func (db *redisDB) SaveBuildLog(job string, build int, log []byte) error {
	conn := db.pool.Get()
	defer conn.Close()

	idStr := fmt.Sprintf("%d", build)

	_, err := conn.Do("SET", "logs:"+job+":"+idStr, log)
	return err
}

func (db *redisDB) GetCurrentSource(job, input string) (config.Source, error) {
	conn := db.pool.Get()
	defer conn.Close()

	sourceBytes, err := redis.Bytes(conn.Do("GET", "current_source:"+job+":"+input))
	if err != nil {
		return nil, err
	}

	return config.Source(sourceBytes), nil
}

func (db *redisDB) SaveCurrentSource(job, input string, source config.Source) error {
	conn := db.pool.Get()
	defer conn.Close()

	_, err := conn.Do("SET", "current_source:"+job+":"+input, []byte(source))
	return err
}

func (db *redisDB) SaveOutputSource(job string, build int, resourceName string, source config.Source) error {
	conn := db.pool.Get()
	defer conn.Close()

	_, err := conn.Do("ZADD", "output:"+job+":"+resourceName, strconv.Itoa(build), []byte(source))
	return err
}

func (db *redisDB) GetCommonOutputs(jobs []string, resourceName string) ([]config.Source, error) {
	conn := db.pool.Get()
	defer conn.Close()

	commonKey := strings.Join(append(append([]string{"common_outputs"}, jobs...), resourceName), ":")

	outputKeys := []interface{}{}
	for _, job := range jobs {
		outputKeys = append(outputKeys, "output:"+job+":"+resourceName)
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
