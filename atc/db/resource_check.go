package db

//go:generate counterfeiter . ResourceCheck

type ResourceCheck interface {
	ID() int
}
