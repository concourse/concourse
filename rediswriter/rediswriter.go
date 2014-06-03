package rediswriter

import (
	"io"

	"github.com/winston-ci/winston/db"
)

type writer struct {
	job   string
	build int

	db db.DB
}

func NewWriter(job string, build int, db db.DB) io.Writer {
	return &writer{
		job:   job,
		build: build,
		db:    db,
	}
}

func (writer *writer) Write(data []byte) (int, error) {
	err := writer.db.AppendBuildLog(writer.job, writer.build, data)
	if err != nil {
		return 0, err
	}

	return len(data), nil
}
