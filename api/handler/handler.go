package handler

import (
	"sync"

	"github.com/concourse/atc/api/drainer"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/logfanout"
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
