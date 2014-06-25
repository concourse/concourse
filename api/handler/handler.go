package handler

import (
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/logfanout"
)

type Handler struct {
	logger lager.Logger

	db      db.DB
	tracker *logfanout.Tracker
}

func NewHandler(logger lager.Logger, db db.DB, tracker *logfanout.Tracker) *Handler {
	return &Handler{
		logger: logger,

		db:      db,
		tracker: tracker,
	}
}
