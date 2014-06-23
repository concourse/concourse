package handler

import (
	"sync"

	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc/api/drainer"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/logfanout"
)

type Handler struct {
	logger lager.Logger

	db    db.DB
	drain *drainer.Drainer

	logs      map[string]*logfanout.LogFanout
	logsMutex *sync.RWMutex
}

func NewHandler(logger lager.Logger, db db.DB, drain *drainer.Drainer) *Handler {
	return &Handler{
		logger: logger,

		db:    db,
		drain: drain,

		logs:      make(map[string]*logfanout.LogFanout),
		logsMutex: new(sync.RWMutex),
	}
}
