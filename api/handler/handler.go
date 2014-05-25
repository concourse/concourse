package handler

import (
	"sync"

	"github.com/winston-ci/logbuffer"
	"github.com/winston-ci/winston/api/drainer"
	"github.com/winston-ci/winston/db"
)

type Handler struct {
	db    db.DB
	drain *drainer.Drainer

	logs      map[string]*logbuffer.LogBuffer
	logsMutex *sync.RWMutex
}

func NewHandler(db db.DB, drain *drainer.Drainer) *Handler {
	return &Handler{
		db:    db,
		drain: drain,

		logs:      make(map[string]*logbuffer.LogBuffer),
		logsMutex: new(sync.RWMutex),
	}
}
