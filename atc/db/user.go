package db

import (
	"time"
)

type user struct {
	id        int
	sub       string
	name      string
	connector string
	lastLogin time.Time
}

//go:generate counterfeiter . User

type User interface {
	ID() int
	Sub() string
	Name() string
	Connector() string
	LastLogin() time.Time
}

func (u user) ID() int              { return u.id }
func (u user) Sub() string          { return u.sub }
func (u user) Name() string         { return u.name }
func (u user) Connector() string    { return u.connector }
func (u user) LastLogin() time.Time { return u.lastLogin }
