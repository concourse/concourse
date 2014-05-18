package db

import (
	"fmt"
	"strconv"

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
		"Status", fmt.Sprintf("%d", builds.BuildStatusPending),
	)

	if _, err := conn.Do("EXEC"); err != nil {
		return builds.Build{}, err
	}

	return builds.Build{
		ID:     id,
		Status: builds.BuildStatusPending,
	}, nil
}

func (db *redisDB) SaveBuildStatus(job string, id int, state builds.BuildStatus) (builds.Build, error) {
	conn := db.pool.Get()
	defer conn.Close()

	idStr := fmt.Sprintf("%d", id)

	err := conn.Send("MULTI")
	if err != nil {
		return builds.Build{}, err
	}

	err = conn.Send("HSET", "build:"+job+":"+idStr, "Status", fmt.Sprintf("%d", state))
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
