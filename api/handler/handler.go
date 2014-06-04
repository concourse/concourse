package handler

import (
	"sync"

	"github.com/winston-ci/winston/api/drainer"
	"github.com/winston-ci/winston/db"
	"github.com/winston-ci/winston/logfanout"
)

type Handler struct {
	db    db.DB
	drain *drainer.Drainer

	logs      map[string]*logfanout.LogFanout
	logsMutex *sync.RWMutex
}

func NewHandler(db db.DB, drain *drainer.Drainer) *Handler {
	return &Handler{
		db:    db,
		drain: drain,

		logs:      make(map[string]*logfanout.LogFanout),
		logsMutex: new(sync.RWMutex),
	}
}
